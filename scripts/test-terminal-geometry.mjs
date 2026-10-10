#!/usr/bin/env node
// Runs ui/js/desktop/apps/terminal-geometry.js (after terminal-styles.js) in Node with a fake xterm, glass and
// animation frames: the three modes of Contract C (fit, bbs 80x25 "Aura VGA", c64 40x25 "Aura C64"), the font
// load before the first fit, the 0.5 px fit loop and its centring padding, one fit at a time, the pinned font
// re-applied at once after a style change, the return to the fit addon with the style's options, and dispose.
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
const window = {
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

// xterm stand-in: the grid measures cols x cell width and rows x line height of the current font size; `slack`
// adds pixels per row, like a font whose measured cell is taller than its em.
function setup() {
    const attrs = new Map();
    const root = { setAttribute: (name, value) => attrs.set(name, String(value)), removeAttribute: (name) => attrs.delete(name) };
    const screen = { clientWidth: 1000, clientHeight: 600 };
    const grid = { style: styleBox(), slack: 0 };
    const term = {
        cols: 100,
        rows: 30,
        resizes: [],
        options: { fontFamily: 'ui-monospace', fontSize: 13, lineHeight: 1.2, scrollback: 1000, theme: { background: '#123456' } },
        resize(cols, rows) { this.cols = cols; this.rows = rows; this.resizes.push(cols + 'x' + rows); },
        element: { style: styleBox(), querySelector: (selector) => (selector === '.xterm-screen' ? grid : null) }
    };
    grid.getBoundingClientRect = () => {
        const family = term.options.fontFamily;
        const em = family.startsWith('"Aura VGA"') ? 0.5 : family.startsWith('"Aura C64"') ? 1 : 0.6;
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
    same('bbs pins 80x25 with the VGA font and no scrollback', [term.resizes, term.options.fontFamily.split(',')[0], term.options.lineHeight, term.options.scrollback], [['80x25'], '"Aura VGA"', 1, 0]);
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

// 4. c64: 40x25 in the C64 font; a taller measured cell shrinks the size until the grid fits.
{
    const { geometry, term, attrs, grid } = setup();
    fontLoads.length = 0;
    geometry.set('c64');
    same('c64 marks the root', attrs.get('data-terminal-geometry'), 'c64');
    same('c64 pins 40x25 with the C64 font', [term.resizes, term.options.fontFamily.split(',')[0], term.options.scrollback], [['40x25'], '"Aura C64"', 0]);
    same('the C64 font is loaded before the first fit', fontLoads, ['16px "Aura C64"']);
    await settle();
    same('C64 cells are one em square: 22 px fits', [term.options.fontSize, padding(term.element.style)], [22, ['25px', '60px', '25px', '60px']]);
    grid.slack = 1;
    geometry.set('c64');
    await settle();
    same('entering again refits; the loop steps down in 0.5 px', term.options.fontSize, 21);
    same('the smaller grid is centred again', padding(term.element.style), ['25px', '80px', '25px', '80px']);
    geometry.set('bbs');
    same('switching pinned modes clears the old letterbox first', [attrs.get('data-terminal-geometry'), term.cols, term.rows], ['bbs', 80, 25]);
    await settle();
}

// 5. Back to fit: attribute, letterbox and scrollback reset, the style's options, the fit addon, a report.
{
    const { geometry, term, attrs, grid, fitAddon, events } = setup();
    geometry.set('c64');
    await settle();
    const before = events.fitted;
    geometry.set('fit');
    same('fit removes the attribute', attrs.has('data-terminal-geometry'), false);
    same('fit clears the letterbox', [term.element.style.padding, grid.style.background], ['', '']);
    const amber = Styles.profile('amber');
    same('fit restores scrollback and the style options', [term.options.scrollback, term.options.fontFamily, term.options.fontSize, term.options.lineHeight], [1000, amber.fontFamily, amber.fontSize, amber.lineHeight]);
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
