#!/usr/bin/env node
// Runs ui/js/desktop/apps/terminal-geometry.js (after terminal-styles.js) in Node with a fake xterm, glass and
// animation frames: the three modes of Contract C (fit, bbs 80x25 "Aura VGA", c64 40x25 "Aura C64"), the font
// load before the first fit, the 0.5 px fit loop and its centring padding, the C64 grid scaled by whole device
// pixels at several DPRs (with xterm's rounded cell corrected or stepped down) and its 0.5 px fallback, the refit
// on a DPR change, one fit at a time, the pinned font re-applied at once after a style change, the return to the
// fit addon with the style's options, and dispose.
// Usage: node scripts/test-terminal-geometry.mjs
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const apps = path.join(here, '..', 'ui', 'js', 'desktop', 'apps');

let failures = 0;
function check(name, cond, detail) {
    if (cond) {
        console.log('ok   ' + name);
        return;
    }
    failures += 1;
    console.log('FAIL ' + name + (detail ? ' - ' + detail : ''));
}
const same = (name, actual, expected) => check(name, JSON.stringify(actual) === JSON.stringify(expected), JSON.stringify(actual) + ' !== ' + JSON.stringify(expected));

// Animation frames run only when the test pumps them.
let frames = new Map();
let frameId = 0;
const fontLoads = [];
let computedReads = 0;
// Media queries the module watches; a test fires `change` on them after changing devicePixelRatio.
const mediaQueries = [];
const window = {
    devicePixelRatio: 1,
    matchMedia: (query) => {
        const listeners = new Set();
        const mq = { query, listeners, addEventListener: (type, fn) => listeners.add(fn), removeEventListener: (type, fn) => listeners.delete(fn) };
        mediaQueries.push(mq);
        return mq;
    },
    localStorage: { getItem: () => null, setItem() {} },
    requestAnimationFrame: (fn) => { frameId += 1; frames.set(frameId, fn); return frameId; },
    cancelAnimationFrame: (id) => { frames.delete(id); },
    // The style's own padding (20px on every side); inline letterbox padding is cleared before this is read.
    getComputedStyle: () => { computedReads += 1; return { paddingLeft: '20px', paddingRight: '20px', paddingTop: '20px', paddingBottom: '20px' }; }
};
const document = { fonts: { load: (spec) => { fontLoads.push(spec); return Promise.resolve([{ status: 'loaded' }]); } } };
const context = vm.createContext({ window, document, Promise, Math, Object, String, Number, parseFloat, JSON });
for (const file of ['terminal-styles.js', 'terminal-geometry.js']) {
    vm.runInContext(fs.readFileSync(path.join(apps, file), 'utf8'), context, { filename: file });
}
const Geometry = window.TerminalGeometry;
const Styles = window.TerminalStyles;

async function settle() {
    for (let round = 0; round < 200; round += 1) {
        await new Promise((resolve) => setImmediate(resolve));
        if (!frames.size) {
            await new Promise((resolve) => setImmediate(resolve));
            if (!frames.size) return;
        }
        const due = [...frames.values()];
        frames = new Map();
        due.forEach((fn) => fn(round * 16));
    }
    throw new Error('frames never settled');
}

function styleBox() {
    const style = { paddingTop: '', paddingRight: '', paddingBottom: '', paddingLeft: '', background: '' };
    Object.defineProperty(style, 'padding', {
        get() { return [style.paddingTop, style.paddingRight, style.paddingBottom, style.paddingLeft].join(' ').trim(); },
        set(value) { style.paddingTop = style.paddingRight = style.paddingBottom = style.paddingLeft = value; }
    });
    return style;
}

// xterm 6 (WebGL) cell for C64 Pro Mono in Chrome, as measured in headless Chrome at DPR 1 to 3: the advance is
// truncated to 1/65536 px and floored in device px, the font box is the rounded ascent (7/8 em) plus the rounded
// descent (1/8 em) in CSS px, ceiled in device px; the line height multiplies it (floored), letter spacing adds
// whole device px, and the canvas size is rounded back to CSS px. `slack` adds device px per row.
function c64Box(term, slack) {
    const o = term.options;
    const dpr = window.devicePixelRatio;
    const round = (x) => Math.floor(x + 0.5);
    const w = Math.floor(Math.floor(o.fontSize * 65536) / 65536 * dpr) + Math.round(o.letterSpacing || 0);
    const h = Math.floor(Math.ceil((round(o.fontSize * 7 / 8) + round(o.fontSize / 8)) * dpr) * o.lineHeight) + slack;
    return { width: Math.round(term.cols * w / dpr), height: Math.round(term.rows * h / dpr) };
}

// xterm stand-in: the C64 font per c64Box; other fonts measure cols x cell width and rows x line height of the
// current font size, with `slack` pixels per row, like a font whose measured cell is taller than its em.
function setup() {
    const attrs = new Map();
    const root = { setAttribute: (name, value) => attrs.set(name, String(value)), removeAttribute: (name) => attrs.delete(name) };
    const screen = { clientWidth: 1000, clientHeight: 600 };
    const grid = { style: styleBox(), slack: 0 };
    const term = {
        cols: 100,
        rows: 30,
        resizes: [],
        sizes: [],
        options: { fontFamily: 'ui-monospace', lineHeight: 1.2, scrollback: 1000, allowTransparency: false, theme: { background: '#123456' } },
        resize(cols, rows) { this.cols = cols; this.rows = rows; this.resizes.push(cols + 'x' + rows); },
        element: { style: styleBox(), querySelector: (selector) => (selector === '.xterm-screen' ? grid : null) }
    };
    // Every font size written, in order (xterm measures the cell again on each change).
    let fontSize = 13;
    Object.defineProperty(term.options, 'fontSize', { enumerable: true, get: () => fontSize, set: (value) => { fontSize = value; term.sizes.push(value); } });
    grid.getBoundingClientRect = () => {
        const family = term.options.fontFamily;
        if (family.startsWith('"Aura C64"')) return c64Box(term, grid.slack);
        const em = family.startsWith('"Aura VGA"') ? 0.5 : 0.6;
        return { width: term.cols * em * term.options.fontSize, height: term.rows * (term.options.fontSize * term.options.lineHeight + grid.slack) };
    };
    const fitAddon = { calls: 0, fit() { this.calls += 1; } };
    const events = { fitted: 0 };
    const geometry = Geometry.create({
        term, screen, root, fit: fitAddon,
        getProfile: () => Styles.profile('amber'),
        onFitted: () => { events.fitted += 1; }
    });
    return { attrs, screen, grid, term, fitAddon, events, geometry };
}
const padding = (style) => [style.paddingTop, style.paddingRight, style.paddingBottom, style.paddingLeft];

// 1. The mode table.
same('modes', Object.keys(Geometry.MODES), ['fit', 'bbs', 'c64']);
same('fit has no pinned grid', Geometry.MODES.fit, null);
same('bbs is 80x25 in the VGA font', [Geometry.MODES.bbs.cols, Geometry.MODES.bbs.rows, Geometry.MODES.bbs.font.split(',')[0]], [80, 25, '"Aura VGA"']);
same('c64 is 40x25 in the C64 font', [Geometry.MODES.c64.cols, Geometry.MODES.c64.rows, Geometry.MODES.c64.font.split(',')[0]], [40, 25, '"Aura C64"']);
same('the module exposes only its contract', Object.keys(Geometry), ['MODES', 'create']);

// 2. Fit mode follows the fit addon.
{
    const { geometry, fitAddon, attrs } = setup();
    same('starts in fit mode', geometry.mode(), 'fit');
    geometry.refit();
    same('refit in fit mode runs the fit addon', fitAddon.calls, 1);
    geometry.set('fit');
    same('fit again changes nothing', [fitAddon.calls, attrs.has('data-terminal-geometry')], [1, false]);
    geometry.set('toString');
    same('unknown modes are fit', [geometry.mode(), attrs.has('data-terminal-geometry')], ['fit', false]);
    same('the returned object has exactly set, mode, refit and dispose', Object.keys(geometry), ['set', 'mode', 'refit', 'dispose']);
}

// 3. bbs: pinned grid, font load, fit, letterbox; a style change gets the pinned font back at once.
{
    const { geometry, term, attrs, grid, events } = setup();
    fontLoads.length = 0;
    geometry.set('bbs');
    same('bbs marks the root', attrs.get('data-terminal-geometry'), 'bbs');
    same('bbs pins 80x25 with the VGA font, no scrollback and the opaque atlas', [term.resizes, term.options.fontFamily.split(',')[0], term.options.lineHeight, term.options.scrollback, term.options.allowTransparency], [['80x25'], '"Aura VGA"', 1, 0, false]);
    same('the VGA font is loaded before the first fit', fontLoads, ['16px "Aura VGA"']);
    await settle();
    same('largest 0.5 px size that fits 960x560 for 40em x 25em', term.options.fontSize, 22);
    same('the grid is centred with inline padding', padding(term.element.style), ['25px', '60px', '25px', '60px']);
    same('the screen paints the theme background', grid.style.background, '#123456');
    check('a changed fit is reported', events.fitted >= 1, String(events.fitted));
    const fitted = events.fitted;
    Styles.applyXterm(term, Styles.profile('green'));
    geometry.refit();
    same('a style change gets the pinned font and fitted size back at once', [term.options.fontFamily.split(',')[0], term.options.fontSize, term.options.lineHeight], ['"Aura VGA"', 22, 1]);
    await settle();
    same('an unchanged refit writes nothing and reports nothing', [padding(term.element.style), events.fitted], [['25px', '60px', '25px', '60px'], fitted]);
    same('the theme background follows the style', grid.style.background, Styles.profile('green').theme.background);
}

// 4. c64: 40x25 in the C64 font, scaled by whole device pixels (8k px cells); a cell xterm makes taller than
// that falls back to the 0.5 px steps.
{
    const { geometry, term, attrs, grid } = setup();
    fontLoads.length = 0;
    geometry.set('c64');
    same('c64 marks the root', attrs.get('data-terminal-geometry'), 'c64');
    same('c64 pins 40x25 with the C64 font', [term.resizes, term.options.fontFamily.split(',')[0], term.options.scrollback], [['40x25'], '"Aura C64"', 0]);
    same('c64 draws from the alpha glyph atlas (no LCD fringe over the neighbouring cell)', term.options.allowTransparency, true);
    same('the C64 font is loaded before the first fit', fontLoads, ['16px "Aura C64"']);
    term.sizes.length = 0;
    await settle();
    same('once the font loaded, a size change makes xterm measure it before the fit', term.sizes.slice(0, 2), [17, 16]);
    same('960x560 at DPR 1: 16 px (k = 2), not the 22 px of 0.5 px steps', [term.options.fontSize, term.options.lineHeight, term.options.letterSpacing], [16, 1, 0]);
    same('the whole-pixel grid is centred, the rest stays letterbox', padding(term.element.style), ['100px', '180px', '100px', '180px']);
    grid.slack = 1;
    geometry.set('c64');
    await settle();
    same('entering again refits; a cell xterm makes taller falls back to the 0.5 px loop', term.options.fontSize, 21);
    same('the fallback grid is centred again', padding(term.element.style), ['25px', '80px', '25px', '80px']);
    geometry.set('bbs');
    same('switching pinned modes clears the old letterbox first', [attrs.get('data-terminal-geometry'), term.cols, term.rows], ['bbs', 80, 25]);
    await settle();
}

// 4b. Whole pixels per glass size and DPR: the fitted size, line height and letter spacing, the device cell xterm
// ends up with (c64Box) and the letterbox padding.
async function c64Fit(dpr, width, height, mode = 'c64') {
    window.devicePixelRatio = dpr;
    const { geometry, term, screen } = setup();
    screen.clientWidth = width + 40;
    screen.clientHeight = height + 40;
    geometry.set(mode);
    await settle();
    const box = c64Box(term, 0);
    const fit = {
        size: term.options.fontSize,
        lineHeight: term.options.lineHeight,
        letterSpacing: term.options.letterSpacing,
        cell: [Math.round(box.width * dpr / 40), Math.round(box.height * dpr / 25)],
        padding: padding(term.element.style)
    };
    geometry.dispose();
    window.devicePixelRatio = 1;
    return fit;
}
const whole = (fit, dpr, cell) => Math.abs(fit.size * dpr - cell) < 1e-9 && cell % 8 === 0 && fit.cell.every((n) => n === cell);
{
    let fit = await c64Fit(1, 750, 473);
    same('750x473 at DPR 1: 16 px, centred', [fit.size, fit.cell, fit.padding], [16, [16, 16], ['56.5px', '75px', '56.5px', '75px']]);
    fit = await c64Fit(1, 1880, 1000);
    same('a maximized 1880x1000 at DPR 1: 40 px (k = 5), the height fills', [fit.size, fit.cell, fit.padding], [40, [40, 40], ['20px', '160px', '20px', '160px']]);
    fit = await c64Fit(1.25, 1000, 700);
    check('1000x700 at DPR 1.25: 19.2 px = 24 device px', whole(fit, 1.25, 24), JSON.stringify(fit));
    same('xterm floors the advance to 23 device px: one px of letter spacing restores 24', [fit.lineHeight, fit.letterSpacing], [1, 1]);
    fit = await c64Fit(1.5, 500, 300);
    check('500x300 at DPR 1.5: 16 device px', whole(fit, 1.5, 16), JSON.stringify(fit));
    same('a 15 px font box gets line height 16.5/15 and the advance one px of spacing', [fit.lineHeight, fit.letterSpacing], [16.5 / 15, 1]);
    fit = await c64Fit(2, 750, 473);
    check('750x473 at DPR 2: 16 px = 32 device px (k = 4)', fit.size === 16 && whole(fit, 2, 32), JSON.stringify(fit));
    fit = await c64Fit(2, 900, 560);
    check('900x560 at DPR 2: k = 5 rounds to a 42 px cell, so k = 4 (32 px)', fit.size === 16 && whole(fit, 2, 32), JSON.stringify(fit));
    fit = await c64Fit(1.25, 750, 473);
    same('750x473 at DPR 1.25: no k up to 2 gets an exact cell from xterm, so the 0.5 px loop', [fit.size, fit.lineHeight, fit.letterSpacing], [18.5, 1, 0]);
    fit = await c64Fit(1, 260, 160);
    same('a glass below 320x200 at DPR 1 (k = 1 does not fit) falls back to the 0.5 px loop', [fit.size, fit.padding], [6, ['25px', '30px', '25px', '30px']]);
    fit = await c64Fit(1.25, 960, 560, 'bbs');
    same('bbs keeps its 0.5 px loop at any DPR', [fit.size, fit.lineHeight, fit.letterSpacing], [22, 1, 0]);
}

// 4c. A DPR change refits a pinned grid: the module watches (resolution: <dpr>dppx), re-arms for the new ratio,
// and dispose stops watching.
{
    const { geometry, term } = setup();
    const first = mediaQueries.at(-1);
    same('the current DPR is watched', [first.query, first.listeners.size], ['(resolution: 1dppx)', 1]);
    geometry.set('c64');
    await settle();
    window.devicePixelRatio = 1.25;
    [...first.listeners].forEach((fn) => fn());
    await settle();
    const second = mediaQueries.at(-1);
    same('the change refits 960x560 at DPR 1.25: 19.2 px with one px of spacing', [term.options.fontSize, term.options.letterSpacing], [19.2, 1]);
    same('the old query is released, the new ratio watched', [first.listeners.size, second.query, second.listeners.size], [0, '(resolution: 1.25dppx)', 1]);
    geometry.dispose();
    same('dispose stops watching', second.listeners.size, 0);
    window.devicePixelRatio = 1;
}

// 5. Back to fit: attribute, letterbox, scrollback and letter spacing reset, the style's options, the fit addon,
// a report. At DPR 1.25 the C64 grid had one px of spacing.
{
    window.devicePixelRatio = 1.25;
    const { geometry, term, attrs, grid, fitAddon, events } = setup();
    geometry.set('c64');
    await settle();
    window.devicePixelRatio = 1;
    same('the C64 grid had letter spacing', term.options.letterSpacing, 1);
    const before = events.fitted;
    geometry.set('fit');
    same('fit removes the attribute', attrs.has('data-terminal-geometry'), false);
    same('fit clears the letterbox', [term.element.style.padding, grid.style.background], ['', '']);
    const amber = Styles.profile('amber');
    same('fit restores scrollback, no letter spacing, the opaque atlas and the style options', [term.options.scrollback, term.options.letterSpacing, term.options.allowTransparency, term.options.fontFamily, term.options.fontSize, term.options.lineHeight], [1000, 0, false, amber.fontFamily, amber.fontSize, amber.lineHeight]);
    same('fit runs the fit addon and reports', [fitAddon.calls, events.fitted], [1, before + 1]);
    same('mode is fit', geometry.mode(), 'fit');
}

// 6. One fit at a time: a refit during a fit reruns it once afterwards; leaving during a fit leaves no padding.
{
    const { geometry, term } = setup();
    geometry.set('bbs');
    await new Promise((resolve) => setImmediate(resolve));
    const reads = computedReads;
    frames.forEach((fn) => fn(0));
    frames = new Map();
    await new Promise((resolve) => setImmediate(resolve));
    geometry.refit();
    geometry.refit();
    await settle();
    same('two refits during a fit rerun it exactly once', computedReads - reads, 1);
    geometry.set('bbs');
    geometry.set('fit');
    await settle();
    same('a fit superseded by leaving writes no padding', term.element.style.padding, '');
}

// 7. dispose stops everything; a zero-sized glass is never fitted.
{
    const { geometry, term } = setup();
    geometry.set('c64');
    geometry.dispose();
    const reads = computedReads;
    await settle();
    same('no fit after dispose', computedReads, reads);
    geometry.set('bbs');
    same('set after dispose does nothing', term.resizes, ['40x25']);
    const hidden = setup();
    hidden.screen.clientWidth = 0;
    hidden.geometry.set('bbs');
    await settle();
    same('a zero-sized glass gets no padding', hidden.term.element.style.padding, '');
}

if (failures) {
    console.log(failures + ' check(s) failed');
    process.exit(1);
}
console.log('terminal geometry ok');
