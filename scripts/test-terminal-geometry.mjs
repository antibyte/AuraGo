#!/usr/bin/env node
// Runs ui/js/desktop/apps/terminal-geometry.js (after terminal-styles.js) in Node with a fake xterm, glass and
// animation frames: the three modes of Contract C (fit, bbs 80x25 "Aura VGA", c64 40x25 "Aura C64"), the font
// load before the first fit, the 0.5 px fit loop and its centring padding, the C64 grid scaled by whole device
// pixels at several DPRs (with xterm's rounded cell corrected or stepped down) and its 0.5 px fallback, the refit
// on a DPR change, one fit at a time, the pinned font re-applied at once after a style change, the return to the
// fit addon with the style's options, dispose, and 'fit' with a minimum grid (80x24 for classic Retro-Net services:
// the style's font shrunk in 0.5 px steps, at once, MIN_FONT_SIZE at the least, restored on leaving).
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
// whole device px, and the canvas size is rounded back to CSS px. `slack` adds device px per row. The DOM renderer
// (`dom`, after a lost WebGL context) does not floor the advance.
function c64Box(term, slack, dom) {
    const o = term.options;
    const dpr = window.devicePixelRatio;
    const round = (x) => Math.floor(x + 0.5);
    const advance = Math.floor(o.fontSize * 65536) / 65536 * dpr;
    const w = (dom ? advance : Math.floor(advance)) + Math.round(o.letterSpacing || 0);
    const h = Math.floor(Math.ceil((round(o.fontSize * 7 / 8) + round(o.fontSize / 8)) * dpr) * o.lineHeight) + slack;
    return { width: Math.round(term.cols * w / dpr), height: Math.round(term.rows * h / dpr) };
}

// xterm stand-in: the C64 font per c64Box; other fonts measure cols x cell width and rows x line height of the
// current font size, with `slack` pixels per row, like a font whose measured cell is taller than its em. The
// module reads offsetWidth/Height; getBoundingClientRect is scaled by `transform` (a window animation).
function setup() {
    const attrs = new Map();
    const root = { setAttribute: (name, value) => attrs.set(name, String(value)), removeAttribute: (name) => attrs.delete(name) };
    const screen = { clientWidth: 1000, clientHeight: 600 };
    const grid = { style: styleBox(), slack: 0, dom: false, transform: 1 };
    const term = {
        cols: 100,
        rows: 30,
        resizes: [],
        sizes: [],
        spacings: [],
        options: { fontFamily: 'ui-monospace', lineHeight: 1.2, scrollback: 1000, allowTransparency: false, theme: { background: '#123456' } },
        resize(cols, rows) { this.cols = cols; this.rows = rows; this.resizes.push(cols + 'x' + rows); },
        element: { style: styleBox(), querySelector: (selector) => (selector === '.xterm-screen' ? grid : null) }
    };
    // Every font size and letter spacing written, in order (xterm measures or rebuilds on each change).
    let fontSize = 13;
    let letterSpacing = 0;
    Object.defineProperty(term.options, 'fontSize', { enumerable: true, get: () => fontSize, set: (value) => { fontSize = value; term.sizes.push(value); } });
    Object.defineProperty(term.options, 'letterSpacing', { enumerable: true, get: () => letterSpacing, set: (value) => { letterSpacing = value; term.spacings.push(value); } });
    const box = () => {
        const family = term.options.fontFamily;
        if (family.startsWith('"Aura C64"')) return c64Box(term, grid.slack, grid.dom);
        const em = family.startsWith('"Aura VGA"') ? 0.5 : 0.6;
        return { width: term.cols * em * term.options.fontSize, height: term.rows * (term.options.fontSize * term.options.lineHeight + grid.slack) };
    };
    Object.defineProperty(grid, 'offsetWidth', { get: () => Math.round(box().width) });
    Object.defineProperty(grid, 'offsetHeight', { get: () => Math.round(box().height) });
    grid.getBoundingClientRect = () => ({ width: box().width * grid.transform, height: box().height * grid.transform });
    // The fit addon: whole cells of the current font in the glass minus the style's 20px padding and, with
    // scrollback, the 14px scrollbar (as xterm's addon-fit); `proposals` counts the measurements.
    const fitAddon = {
        calls: 0,
        proposals: 0,
        proposeDimensions() {
            this.proposals += 1;
            const cell = { width: 0.6 * term.options.fontSize, height: term.options.fontSize * term.options.lineHeight + grid.slack };
            const width = screen.clientWidth - 40 - (term.options.scrollback ? 14 : 0);
            return { cols: Math.max(2, Math.floor(width / cell.width)), rows: Math.max(1, Math.floor((screen.clientHeight - 40) / cell.height)) };
        },
        fit() {
            this.calls += 1;
            const dims = this.proposeDimensions();
            if (term.cols !== dims.cols || term.rows !== dims.rows) term.resize(dims.cols, dims.rows);
        }
    };
    const events = { fitted: 0 };
    const style = { id: 'amber' };
    const geometry = Geometry.create({
        term, screen, root, fit: fitAddon,
        getProfile: () => Styles.profile(style.id),
        onFitted: () => { events.fitted += 1; }
    });
    return { attrs, screen, grid, term, fitAddon, events, geometry, style };
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
    grid.slack = -1;
    geometry.set('c64');
    await settle();
    same('a cell one device px short in height is not nudged by line height (glyphs would drop a px): 0.5 px loop', [term.options.fontSize, term.options.lineHeight], [22, 1]);
    grid.slack = 0;
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
    fit = await c64Fit(1.5, 750, 473);
    check('750x473 at DPR 1.5: 16 px = 24 device px (k = 3)', fit.size === 16 && fit.letterSpacing === 0 && whole(fit, 1.5, 24), JSON.stringify(fit));
    fit = await c64Fit(1.5, 500, 300);
    same('500x300 at DPR 1.5: k = 2 gets a 15 px font box (never nudged), k = 1 a 9 px one: the 0.5 px loop', [fit.size, fit.lineHeight, fit.letterSpacing], [11.5, 1, 0]);
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

// 4d. The letter spacing at DPR 1.25: a refit at the same k never passes through other cells, the measure ignores
// CSS transforms, the DOM renderer (lost WebGL context) gets its spacing back to 0, and no other grid inherits it.
{
    window.devicePixelRatio = 1.25;
    const { geometry, term, grid } = setup();
    geometry.set('c64');
    await settle();
    same('960x560 at DPR 1.25: 19.2 px with one px of spacing', [term.options.fontSize, term.options.letterSpacing], [19.2, 1]);
    term.sizes.length = 0;
    term.spacings.length = 0;
    grid.transform = 0.5;
    geometry.refit();
    await settle();
    check('a refit at the same k (mid window animation) writes only the fitted size and spacing', term.sizes.every((n) => n === 19.2) && term.spacings.every((n) => n === 1),
        JSON.stringify([term.sizes, term.spacings]));
    grid.transform = 1;
    grid.dom = true;
    geometry.refit();
    await settle();
    const dom = c64Box(term, 0, true);
    same('the DOM renderer does not floor the advance: spacing 0, cells still 24 device px', [term.options.fontSize, term.options.letterSpacing, Math.round(dom.width * 1.25 / 40)], [19.2, 0, 24]);
    grid.dom = false;
    geometry.refit();
    await settle();
    same('back on WebGL the spacing returns', term.options.letterSpacing, 1);
    geometry.set('bbs');
    same('switching to bbs drops the C64 spacing at once', term.options.letterSpacing, 0);
    await settle();
    geometry.set('c64');
    await settle();
    geometry.set('fit');
    geometry.set('bbs');
    same('leaving C64 for fit and then entering bbs starts without spacing', term.options.letterSpacing, 0);
    await settle();
    geometry.dispose();
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

// 8. 'fit' with a minimum grid (classic Retro-Net services, 80x24): the style's font size when the fit addon gets
// at least 80x24 from it, else the largest 0.5 px step below it that does, MIN_FONT_SIZE at the least; at once,
// so a session opens with the final grid, and starting at the current size so that a font size is written only
// where the font must change. The fake fit addon measures amber (17 px, line height 1.15, 0.6 em cells) in the
// glass minus 40px padding and the 14px scrollbar.
const CLASSIC = { minCols: 80, minRows: 24 };
const fitsAt = (size, setupState) => {
    const { term, fitAddon } = setupState;
    const kept = term.options.fontSize;
    term.options.fontSize = size;
    const dims = fitAddon.proposeDimensions();
    term.options.fontSize = kept;
    return dims.cols >= 80 && dims.rows >= 24;
};
function inFitWindow(width, height) {
    const state = setup();
    state.screen.clientWidth = width;
    state.screen.clientHeight = height;
    Styles.applyXterm(state.term, Styles.profile('amber'));
    state.fitAddon.fit();
    return state;
}
{
    const state = inFitWindow(760, 420);
    const { geometry, term, fitAddon, attrs, events } = state;
    same('a small window: today\'s fit at 17 px gives 69x19', [term.options.fontSize, term.cols, term.rows], [17, 69, 19]);
    const before = events.fitted;
    const proposals = fitAddon.proposals;
    term.sizes.length = 0;
    geometry.set('fit', CLASSIC);
    same('the minimum grid shrinks the font to 13.5 px at once (no frame): 87x24', [term.options.fontSize, term.cols, term.rows], [13.5, 87, 24]);
    same('one font write: the current 17 px is probed as it is, then the expected 13.5 px, then the fit', [fitAddon.proposals - proposals, term.sizes], [3, [13.5]]);
    check('13.5 px is the largest 0.5 px step that keeps 80x24', fitsAt(13.5, state) && !fitsAt(14, state));
    same('fit mode stays fit, no pinned attribute, scrollback kept', [geometry.mode(), attrs.has('data-terminal-geometry'), term.options.scrollback], ['fit', false, 1000]);
    same('the changed size is reported', events.fitted, before + 1);
    await settle();
    same('nothing follows on a later frame', [term.options.fontSize, term.cols, term.rows], [13.5, 87, 24]);
    // Drag-resizing: each ResizeObserver tick refits; a font size is written only where the font must change.
    const tick = (width, height) => {
        state.screen.clientWidth = width;
        state.screen.clientHeight = height;
        term.sizes.length = 0;
        const calls = fitAddon.calls;
        geometry.refit();
        return [term.sizes.slice(), term.options.fontSize, term.cols, term.rows, fitAddon.calls - calls];
    };
    same('a refit in the same glass writes no font size', tick(760, 420), [[], 13.5, 87, 24, 1]);
    same('5px taller: 13.5 px still fits and 14 px is not expected to, so nothing is written', tick(760, 425), [[], 13.5, 87, 24, 1]);
    same('10px taller: 14 px is expected to fit and does, one write', tick(760, 430), [[14], 14, 84, 24, 1]);
    same('back to 420px: one write down to 13.5 px', tick(760, 420), [[13.5], 13.5, 87, 24, 1]);
    same('a hidden (zero-size) glass keeps the size and grid: no write, no fit', tick(0, 420), [[], 13.5, 87, 24, 0]);
    same('shown again at its size: nothing to write', tick(760, 420), [[], 13.5, 87, 24, 1]);
    same('a large glass gets the style\'s 17 px back with more rows, one write', tick(1400, 900), [[17], 17, 131, 43, 1]);
    same('a large glass that keeps the style\'s size writes nothing', tick(1400, 880), [[], 17, 131, 42, 1]);
    same('shrinking the window shrinks the font again, one write', tick(760, 420), [[13.5], 13.5, 87, 24, 1]);
    state.style.id = 'modern';
    Styles.applyXterm(term, Styles.profile('modern'));
    geometry.refit();
    same('a style change re-evaluates: modern (13 px, line height 1.2) keeps its size at 90x24', [term.options.fontSize, term.cols, term.rows], [13, 90, 24]);
    state.style.id = 'amber';
    Styles.applyXterm(term, Styles.profile('amber'));
    geometry.refit();
    same('back to amber shrinks to 13.5 px again', term.options.fontSize, 13.5);
    const fitted = events.fitted;
    geometry.set('fit');
    same('leaving (directory, shell) restores the style font and today\'s fit', [term.options.fontSize, term.cols, term.rows, events.fitted], [17, 69, 19, fitted + 1]);
    geometry.refit();
    same('without a minimum a refit keeps the style font', [term.options.fontSize, term.rows], [17, 19]);
}
{
    const { geometry, term } = inFitWindow(1400, 900);
    geometry.set('fit', CLASSIC);
    same('a large window keeps the style font and gets more than 80x24', [term.options.fontSize, term.cols, term.rows], [17, 131, 43]);
    geometry.set('fit');
    same('leaving a minimum that never shrank changes nothing', [term.options.fontSize, term.cols, term.rows], [17, 131, 43]);
}
{
    const { geometry, term } = inFitWindow(300, 200);
    geometry.set('fit', CLASSIC);
    same('a tiny window clamps at MIN_FONT_SIZE (6 px) with whatever fits', [term.options.fontSize, term.cols, term.rows], [6, 68, 23]);
    geometry.set('fit');
    same('leaving restores the style font', term.options.fontSize, 17);
}
{
    // Cells 8 px shorter per row than the font scales (not proportional): the first guess is too small, so the
    // search steps up to the largest size that still fits.
    const state = inFitWindow(1000, 240);
    state.grid.slack = -8;
    state.geometry.set('fit', CLASSIC);
    same('a low first guess steps up to the largest size that fits (14 px, not 12.5)', state.term.options.fontSize, 14);
    check('14 px keeps 80x24 and 14.5 px does not', fitsAt(14, state) && !fitsAt(14.5, state));
}
{
    const { geometry, term, attrs, screen } = inFitWindow(760, 420);
    geometry.set('fit', CLASSIC);
    geometry.set('bbs');
    same('a pinned mode drops the minimum', [attrs.get('data-terminal-geometry'), term.cols, term.rows], ['bbs', 80, 25]);
    await settle();
    geometry.set('fit', CLASSIC);
    same('leaving a pinned mode for fit with a minimum applies it', [attrs.has('data-terminal-geometry'), term.options.fontSize, term.cols, term.rows], [false, 13.5, 87, 24]);
    geometry.set('fit', {});
    same('empty options are no minimum: the style font returns', term.options.fontSize, 17);
    screen.clientWidth = 0;
    geometry.set('fit', CLASSIC);
    same('entering the minimum with a hidden glass keeps the current (style) font', term.options.fontSize, 17);
    screen.clientWidth = 760;
    geometry.set('fit', CLASSIC);
    geometry.dispose();
    geometry.refit();
    geometry.set('fit');
    same('after dispose nothing changes the font', term.options.fontSize, 13.5);
}

if (failures) {
    console.log(failures + ' check(s) failed');
    process.exit(1);
}
console.log('terminal geometry ok');
