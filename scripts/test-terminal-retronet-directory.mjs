#!/usr/bin/env node
// Runs ui/js/desktop/apps/terminal-text.js and terminal-retronet-directory.js against the vendored xterm.js (headless, no DOM, no npm
// packages) and checks that the Retro-Net directory measures text in terminal cells exactly like xterm (CJK, emoji,
// ZWJ sequences, combining marks, Devanagari), never cuts a cluster or surrogate pair, strips C0/C1 control
// characters from server-provided strings and keeps every directory row aligned and inside the screen.
// Usage: node scripts/test-terminal-retronet-directory.mjs
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const ui = path.join(here, '..', 'ui');
const read = (rel) => fs.readFileSync(path.join(ui, rel), 'utf8');

const store = new Map();
const sandbox = {
    console, setTimeout, clearTimeout, setInterval, clearInterval, queueMicrotask, performance,
    navigator: { userAgent: 'node', platform: 'Linux', language: 'en-US', maxTouchPoints: 0 },
    document: { documentElement: { lang: 'en' }, queryCommandSupported: () => false, addEventListener() {}, removeEventListener() {} },
    localStorage: { getItem: (key) => (store.has(key) ? store.get(key) : null), setItem: (key, value) => store.set(key, String(value)) },
    SYSTEM_LANG: 'en'
};
sandbox.window = sandbox;
sandbox.self = sandbox;
sandbox.globalThis = sandbox;
vm.createContext(sandbox);
vm.runInContext(read('js/vendor/xterm.min.js'), sandbox, { filename: 'xterm.min.js' });
vm.runInContext(read('js/desktop/apps/terminal-text.js'), sandbox, { filename: 'terminal-text.js' });
vm.runInContext(read('js/desktop/apps/terminal-retronet-directory.js'), sandbox, { filename: 'terminal-retronet-directory.js' });

const Directory = sandbox.TerminalRetroNetDirectory;
const { cellWidth, fitToCells, printable } = sandbox.TerminalText;
const english = JSON.parse(read('lang/desktop/en.json'));
const t = (key, params) => {
    let text = Object.prototype.hasOwnProperty.call(english, key) ? english[key] : key;
    Object.keys(params || {}).forEach((name) => { text = text.split('{{' + name + '}}').join(String(params[name])); });
    return text;
};
const CONTROL = /[\u0000-\u001f\u007f-\u009f]/;
const LONE_SURROGATE = /[\uD800-\uDBFF](?![\uDC00-\uDFFF])|(?<![\uD800-\uDBFF])[\uDC00-\uDFFF]/;

let failures = 0;
function check(name, cond, detail) {
    if (cond) {
        console.log('ok   ' + name);
        return;
    }
    failures += 1;
    console.log('FAIL ' + name + (detail ? ' - ' + detail : ''));
}
const width = (text) => [...text].reduce((sum, ch) => sum + cellWidth(ch.codePointAt(0)), 0);
const same = (name, actual, expected) => check(name, actual === expected, JSON.stringify(actual) + ' !== ' + JSON.stringify(expected));

// 1. cellWidth is xterm's own Unicode 6 width for every code point (test-only access to xterm's private service).
const probe = new sandbox.Terminal({ cols: 20, rows: 2 });
const xtermWidth = probe._core.unicodeService.wcwidth.bind(probe._core.unicodeService);
const mismatches = [];
for (let code = 0x20; code <= 0x10ffff; code += 1) {
    if ((code >= 0x7f && code < 0xa0) || (code >= 0xd800 && code <= 0xdfff)) continue;
    if (cellWidth(code) !== xtermWidth(code)) mismatches.push(code.toString(16) + ':' + cellWidth(code) + '/' + xtermWidth(code));
}
check('cellWidth matches xterm Unicode 6 for all code points', mismatches.length === 0, mismatches.slice(0, 8).join(' '));
same('vendored xterm uses the Unicode 6 provider (emoji one cell)', probe._core.unicodeService.activeVersion, '6');
probe.dispose();

// 2. fitToCells: ASCII, CJK, emoji, ZWJ, combining marks, flags, skin tones, control characters.
same('ascii fits', fitToCells('Retro', 8), 'Retro');
same('ascii cut', fitToCells('Retro-Net Directory', 8), 'Retro-N\u2026');
same('width 1 is an ellipsis', fitToCells('abc', 1), '\u2026');
same('width 0 is empty', fitToCells('abc', 0), '');
same('cjk fits exactly', fitToCells('\u6f22\u5b57\u30c6\u30b9\u30c8', 10), '\u6f22\u5b57\u30c6\u30b9\u30c8');
same('cjk cut keeps whole wide cells', fitToCells('\u6f22\u5b57\u30c6\u30b9\u30c8', 6), '\u6f22\u5b57\u2026');
same('cjk cut at odd width', fitToCells('\u6f22\u5b57\u30c6\u30b9\u30c8', 9), '\u6f22\u5b57\u30c6\u30b9\u2026');
check('cjk counts two cells', width('\u6f22\u5b57') === 4);
same('emoji are one cell in Unicode 6', fitToCells('\u{1F600}\u{1F600}\u{1F600}\u{1F600}', 3), '\u{1F600}\u{1F600}\u2026');
const family = '\u{1F468}\u200d\u{1F469}\u200d\u{1F467}';
same('zwj family is dropped whole', fitToCells('ab' + family + 'cd', 5), 'ab\u2026');
same('zwj family is kept whole', fitToCells(family + 'xyz', 5), family + 'x\u2026');
same('combining marks stay with their base', fitToCells('e\u0301e\u0301e\u0301e\u0301', 3), 'e\u0301e\u0301\u2026');
same('flags are never split', fitToCells('\u{1F1E9}\u{1F1EA}\u{1F1EB}\u{1F1F7}\u{1F1EE}\u{1F1F9}', 4), '\u{1F1E9}\u{1F1EA}\u2026');
same('skin tones stay with their emoji', fitToCells('\u{1F44D}\u{1F3FD}\u{1F44D}\u{1F3FD}', 3), '\u{1F44D}\u{1F3FD}\u2026');
same('variation selector stays', fitToCells('\u2615\ufe0f\u2615\ufe0f\u2615\ufe0f', 2), '\u2615\ufe0f\u2026');
const samples = ['ab' + family + 'cd' + family, '\u{1F600}x\u{1F600}\u6f22\u{1F44D}\u{1F3FD}', '\u0939\u093f\u0928\u094d\u0926\u0940 \u092c\u094b\u0930\u094d\u0921', '\u{1F1E9}\u{1F1EA}a\u{1F1EB}\u{1F1F7}'];
let clusterBreaks = [];
for (const sample of samples) {
    for (let cells = 0; cells <= 14; cells += 1) {
        {
            const fitted = fitToCells(sample, cells);
            if (width(fitted) > cells) clusterBreaks.push('too wide ' + JSON.stringify(fitted) + '@' + cells);
            if (LONE_SURROGATE.test(fitted)) clusterBreaks.push('lone surrogate ' + JSON.stringify(fitted));
            if (/\u200d(\u2026)?$/.test(fitted)) clusterBreaks.push('dangling zwj ' + JSON.stringify(fitted));
            if (fitted.endsWith('\u2026') && !sample.startsWith(fitted.slice(0, -1))) clusterBreaks.push('not a prefix ' + JSON.stringify(fitted));
        }
    }
}
check('no cut exceeds its width or splits a pair/sequence', clusterBreaks.length === 0, clusterBreaks.slice(0, 4).join('; '));
same('pad fills to the width', fitToCells('\u6f22a', 5, true), '\u6f22a  ');
same('pad after a cut', fitToCells('\u6f22\u5b57\u30c6', 4, true), '\u6f22\u2026 ');
same('pad of nothing', fitToCells('', 3, true), '   ');
same('directory exposes only its contract', Object.keys(Directory).join(','), 'LOCAL_SHELL_ID,create');
same('printable strips C0, DEL and C1', printable('a\u001b[2Jb\u009b31m\u0007c\u007f\u0000d\r\n\u0085e'), 'a[2Jb31mcde');
same('fitToCells strips control characters', fitToCells('\u001b]0;x\u0007ok', 10), ']0;xok');
same('printable keeps zwj and text', printable('\u{1F469}\u200d\u{1F4BB} \u00e9'), '\u{1F469}\u200d\u{1F4BB} \u00e9');

// 3. The rendered directory in a real (headless) xterm: aligned columns, exact row width, no injected escapes.
const hoursAgo = new Date(Date.now() - 3 * 3600 * 1000).toISOString();
const own = (id, name, description) => ({ id, name, description, category: 'own', protocol: 'telnet', host: id + '.example.org', port: 23, kind: 'world', charset: 'utf-8', own: true });
const evilName = 'Evil\u001b[2J\u001b]0;pwned\u0007\u009b31mX';
const payload = {
    entries: [
        { id: 'telehack', name: 'Telehack', category: 'classics', description_key: 'desktop.terminal_retronet_entry_telehack', protocol: 'telnet', host: 'telehack.com', port: 23, kind: 'world', charset: 'utf-8', own: false },
        own('ascii', 'A very long ASCII bulletin board name that never ends', '#ascii ' + 'x'.repeat(200)),
        own('cjk', '\u6f22\u5b57\u306e\u3068\u3066\u3082\u9577\u3044\u63b2\u793a\u677f\u306e\u540d\u524d\u3067\u3059\u672c\u5f53\u306b', '#\u6f22\u5b57\u306e\u8aac\u660e\u6587\u304c\u3068\u3066\u3082\u9577\u3044\u3067\u3059'.repeat(4)),
        own('emoji', '\u{1F600} Smiley ' + '\u{1F600}'.repeat(20), '#emoji \u2615\u{1F600}\u{1F389} '.repeat(10)),
        own('zwj', family + ' Family ' + '\u{1F469}\u200d\u{1F4BB}'.repeat(10), '#zwj ' + (family + ' ').repeat(12)),
        own('combining', '\u00c5ngstr\u00f6m e\u0301e\u0301e\u0301 Zalgo a\u0300\u0301\u0302\u0303 long name', '#combining a\u0300\u0301 '.repeat(12)),
        own('hindi', '\u0939\u093f\u0928\u094d\u0926\u0940 \u092c\u0941\u0932\u0947\u091f\u093f\u0928 \u092c\u094b\u0930\u094d\u0921 \u092a\u094d\u0930\u0923\u093e\u0932\u0940', '#\u0939\u093f\u0928\u094d\u0926\u0940 \u0935\u093f\u0935\u0930\u0923 '.repeat(8)),
        own('evil', evilName, '#\u001b[31mred\u001b[0m text'),
        own('old', 'Old BBS', '#offline board')
    ],
    status: { telehack: { state: 'online' }, ascii: { state: 'online' }, old: { state: 'offline', last_online_at: hoursAgo }, evil: { state: 'bogus' } },
    stale: false,
    can_edit: true
};

function cellsOf(line, cols) {
    const cells = [];
    for (let x = 0; x < cols; x += 1) {
        const cell = line.getCell(x);
        cells.push({ chars: cell.getChars(), width: cell.getWidth(), fgDefault: cell.isFgDefault() });
    }
    return cells;
}

function textAt(cells, from, to) {
    return cells.slice(from, to).map((cell) => cell.chars).join('');
}

async function renderDirectory(cols, rows, translate = t, data = payload) {
    const term = new sandbox.Terminal({ cols, rows });
    const titles = [];
    term.onTitleChange((title) => titles.push(title));
    const announced = [];
    const dialed = [];
    const directory = Directory.create({
        term,
        t: translate,
        api: async (url, options) => {
            if (url === '/api/desktop/retronet/directory') return JSON.parse(JSON.stringify(data));
            if (url === '/api/desktop/retronet/status' && options && options.method === 'POST') return { status: data.status };
            throw new Error('unexpected ' + url);
        },
        announce: (text) => announced.push(text),
        canEdit: () => true,
        onDial: (entry) => dialed.push(entry),
        onEdit() {},
        onDelete() {}
    });
    await directory.load();
    await new Promise((resolve) => term.write('', resolve));
    return { term, directory, titles, announced, dialed };
}

function entryRows(term, cols, rows) {
    const found = [];
    for (let y = 0; y < rows; y += 1) {
        const line = term.buffer.active.getLine(y);
        const text = line.translateToString(true);
        if (/^[> ]\d\d /.test(text)) found.push({ y, text, cells: cellsOf(line, cols) });
    }
    return found;
}

for (const [cols, rows] of [[80, 30], [120, 30], [52, 32]]) {
    const label = cols + 'x' + rows;
    const { term, directory, titles, announced, dialed } = await renderDirectory(cols, rows);
    const found = entryRows(term, cols, rows);
    check(label + ' shows local shell and all entries', found.length === payload.entries.length + 1, 'rows ' + found.length);
    const local = found[0];
    const descStart = local ? local.text.indexOf('Command line') : -1;
    check(label + ' local shell description located', descStart > 8, String(descStart));
    const misaligned = [];
    const overflow = [];
    for (const row of found.slice(1)) {
        const isCatalog = row.text.startsWith(' 01');
        const start = row.cells[descStart];
        if (!isCatalog && start.chars !== '#') misaligned.push(row.text.slice(0, 3) + ' got ' + JSON.stringify(start.chars));
        if (isCatalog && (start.chars === '' || start.chars === ' ')) misaligned.push('catalog description missing');
        if (row.cells[descStart - 1].chars !== ' ') misaligned.push(row.text.slice(0, 3) + ' name overlaps');
        const last = row.cells[cols - 2];
        const edge = row.cells[cols - 1];
        if (!(last.chars !== '' || last.width === 0) || edge.chars !== '') overflow.push(row.text.slice(0, 3) + ' ends ' + JSON.stringify(last.chars) + '|' + JSON.stringify(edge.chars));
    }
    check(label + ' descriptions start in the same cell on every row', misaligned.length === 0, misaligned.join('; '));
    check(label + ' every entry row is exactly cols-1 cells wide', overflow.length === 0, overflow.join('; '));
    const names = found.slice(1).map((row) => textAt(row.cells, 8, descStart - 1));
    const cut = found.slice(1).filter((row) => /^[> ]0[2-6] /.test(row.text)).map((row) => textAt(row.cells, 8, descStart - 1).trim());
    check(label + ' overlong ascii/cjk/emoji/zwj/combining names end in an ellipsis', cut.length === 5 && cut.every((name) => name.endsWith('\u2026')), cut.join(' | '));
    check(label + ' zwj family is whole or absent', names.every((name) => !name.includes('\u{1F468}') || name.includes(family)));
    const evil = found.find((row) => row.text.includes('Evil'));
    check(label + ' control characters from names are stripped', !!evil && evil.text.includes('Evil[2J]0;pwned'.slice(0, Math.min(15, descStart - 10))) && !CONTROL.test(evil.text), evil && evil.text);
    check(label + ' injected escapes did not run', titles.length === 0 && term.buffer.active.getLine(0).translateToString(true).includes('RETRO-NET'));
    const red = evil && evil.cells[descStart + '#[31m'.length];
    check(label + ' injected colour did not apply', !!red && red.chars === 'r' && red.fgDefault, red && JSON.stringify(red));
    const unknownMarker = evil && evil.text.slice(4, 7);
    same(label + ' unknown state shows [?]', unknownMarker, '[?]');
    const old = found.find((row) => row.text.includes('Old BBS'));
    check(label + ' offline entry shows last reachable time', !!old && old.text.slice(4, 7) === '[ ]' && (cols < 60 || old.text.includes('last reachable')), old && old.text);
    const footer = [];
    for (let y = found[found.length - 1].y + 1; y < rows; y += 1) footer.push(term.buffer.active.getLine(y).translateToString(true));
    const helpText = footer.join(' ');
    check(label + ' key help is complete (wrapped, not cut)', helpText.includes('R: check reachability') && helpText.includes('Del: delete'), footer.join(' / '));
    check(label + ' announcements carry no control characters', announced.length > 0 && announced.every((text) => !CONTROL.test(text)), JSON.stringify(announced.at(-1)));

    directory.handleData('\x1b[B');
    same(label + ' arrow down selects 01', directory.selected().id, 'telehack');
    directory.handleData('0');
    same(label + ' digit 0 jumps to the local shell', directory.selected().id, Directory.LOCAL_SHELL_ID);
    directory.handleData('\r');
    check(label + ' enter dials the selection', dialed.length === 1 && dialed[0].id === Directory.LOCAL_SHELL_ID);
    directory.dispose();
    term.dispose();
}

// 4. Every locale with the full catalog: translated descriptions, headings and key help stay inside their cells.
const LANGS = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
const CATALOG_PREFIX = 'desktop.terminal_retronet_entry_';
const catalog = Object.keys(english).filter((key) => key.startsWith(CATALOG_PREFIX)).map((key, index) => ({
    id: key.slice(CATALOG_PREFIX.length), name: 'Catalog entry ' + index, category: ['classics', 'bbs', 'muds', 'games'][index % 4],
    description_key: key, protocol: 'telnet', host: 'bbs.example.org', port: 23, kind: 'bbs', charset: 'cp437', own: false
}));
const fullPayload = { entries: catalog.concat(payload.entries.filter((entry) => entry.own)), status: payload.status, stale: false, can_edit: true };
for (const lang of LANGS) {
    const strings = JSON.parse(read('lang/desktop/' + lang + '.json'));
    const translate = (key, params) => {
        let text = Object.prototype.hasOwnProperty.call(strings, key) ? strings[key] : key;
        Object.keys(params || {}).forEach((name) => { text = text.split('{{' + name + '}}').join(String(params[name])); });
        return text;
    };
    for (const cols of [80, 52]) {
        const rows = 70;
        const label = lang + ' ' + cols + 'x' + rows;
        const { term, announced } = await renderDirectory(cols, rows, translate, fullPayload);
        const found = entryRows(term, cols, rows);
        const descStart = 9 + Math.max(8, Math.min(24, Math.floor((cols - 1) * 0.3)));
        const problems = [];
        if (found.length !== fullPayload.entries.length + 1) problems.push('rows ' + found.length);
        for (const row of found) {
            const start = row.cells[descStart];
            if (row.cells[descStart - 1].chars !== ' ') problems.push(row.text.slice(0, 3) + ' name overlaps');
            if (start.chars === '' || start.chars === ' ') problems.push(row.text.slice(0, 3) + ' description not at ' + descStart);
            if (row.cells[cols - 1].chars !== '') problems.push(row.text.slice(0, 3) + ' overflows');
        }
        for (let y = 0; y < rows; y += 1) {
            const line = term.buffer.active.getLine(y);
            if (!line.translateToString(true).startsWith('\u2500') && line.getCell(cols - 1).getChars() !== '') problems.push('row ' + y + ' reaches the last column');
        }
        check(label + ' rows aligned and inside the screen', problems.length === 0, problems.slice(0, 4).join('; '));
        const screen = [];
        for (let y = 0; y < rows; y += 1) screen.push(term.buffer.active.getLine(y).translateToString(true));
        const hints = (strings['desktop.terminal_retronet_help'] + ' \u00b7 ' + strings['desktop.terminal_retronet_help_admin']).split(' \u00b7 ');
        const missing = hints.filter((hint) => width(hint) <= cols - 1 && !screen.some((text) => text.includes(hint)));
        check(label + ' every key hint is shown whole', missing.length === 0, missing.join(' | '));
        check(label + ' announcements carry no control characters', announced.length > 0 && announced.every((text) => !CONTROL.test(text)));
        term.dispose();
    }
}

// 5. Mouse rows: the directory lives on the alternate screen (no scrollback); a scrolled viewport is still mapped.
const NUMBERED_IDS = [Directory.LOCAL_SHELL_ID, 'telehack', 'ascii', 'cjk', 'emoji', 'zwj', 'combining', 'hindi', 'evil', 'old'];
const flush = (term) => new Promise((resolve) => term.write('', resolve));

function clickable(real, bufferOverride) {
    const listeners = {};
    const element = {
        addEventListener: (type, fn) => { listeners[type] = fn; },
        removeEventListener: (type) => { delete listeners[type]; },
        querySelector: (selector) => (selector === '.xterm-screen'
            ? { getBoundingClientRect: () => ({ top: 100, bottom: 100 + real.rows * 16, height: real.rows * 16 }) }
            : null)
    };
    let writes = 0;
    const term = {
        get cols() { return real.cols; },
        get rows() { return real.rows; },
        get buffer() { return bufferOverride || real.buffer; },
        write: (data, callback) => { writes += 1; return real.write(data, callback); },
        element
    };
    const click = (screenRow, type = 'click', offset = 8) => {
        if (listeners[type]) listeners[type]({ clientY: 100 + screenRow * 16 + offset });
    };
    return { term, click, listeners, writes: () => writes };
}

async function mouseDirectory(term, options = {}) {
    const dialed = [];
    const directory = Directory.create({
        term, t, announce() {}, canEdit: () => true, onEdit() {}, onDelete() {},
        api: async () => JSON.parse(JSON.stringify(options.data || payload)),
        onDial: (entry) => dialed.push(entry.id)
    });
    await directory.load();
    return { directory, dialed };
}

{
    const real = new sandbox.Terminal({ cols: 80, rows: 30, scrollback: 1000 });
    real.write('shell output\r\n'.repeat(60));
    await flush(real);
    const { term, click } = clickable(real);
    const { directory, dialed } = await mouseDirectory(term);
    real.resize(80, 12);
    directory.render();
    await flush(real);
    real.scrollLines(-5);
    const active = real.buffer.active;
    check('directory renders on the alternate screen', active.type === 'alternate' && active.baseY === 0 && active.viewportY === 0, active.type + ' base ' + active.baseY + ' viewport ' + active.viewportY);
    const rowsWithEntries = [];
    for (let row = 0; row < real.rows; row += 1) {
        const match = /^[> ](\d\d) /.exec(active.getLine(active.viewportY + row).translateToString(true));
        if (match) rowsWithEntries.push(row);
    }
    const wrong = [];
    for (const row of rowsWithEntries.slice(1).reverse()) {
        const shown = /^[> ](\d\d) /.exec(real.buffer.active.getLine(real.buffer.active.viewportY + row).translateToString(true));
        click(row);
        await flush(real);
        const want = shown && NUMBERED_IDS[Number(shown[1])];
        if (!want || directory.selected().id !== want) wrong.push('row ' + row + ' shows ' + (shown && shown[1]) + ' selected ' + directory.selected().id);
    }
    check('clicks after shrinking and scrolling select the entry under the pointer', rowsWithEntries.length > 2 && wrong.length === 0 && dialed.length === 0, wrong.join('; ') || String(rowsWithEntries.length));
    directory.dispose();
    real.dispose();
}

{
    const real = new sandbox.Terminal({ cols: 80, rows: 12 });
    const { term, click } = clickable(real, { active: { type: 'alternate', viewportY: 2, baseY: 4 } });
    const { directory } = await mouseDirectory(term);
    click(6);
    same('a viewport scrolled into scrollback is mapped back to the drawn row', directory.selected().id, 'telehack');
    directory.dispose();
    real.dispose();
}

// 6. The dial latch is released by load(); headings upper-case with the UI language.
{
    const real = new sandbox.Terminal({ cols: 80, rows: 30 });
    const { directory, dialed } = await mouseDirectory(clickable(real).term);
    directory.handleData('\r');
    directory.handleData('\r');
    same('a dial latches until the directory reloads', dialed.length, 1);
    await directory.load();
    directory.handleData('\r');
    same('load() releases the dial latch', dialed.length, 2);
    directory.dispose();
    real.dispose();
}
{
    const greek = JSON.parse(read('lang/desktop/el.json'));
    const translate = (key) => (Object.prototype.hasOwnProperty.call(greek, key) ? greek[key] : key);
    const heading = greek['desktop.terminal_retronet_cat_classics'];
    sandbox.SYSTEM_LANG = 'el';
    const { term } = await renderDirectory(80, 30, translate);
    const shown = [];
    for (let y = 0; y < 30; y += 1) shown.push(term.buffer.active.getLine(y).translateToString(true));
    check('Greek headings drop accents when upper-cased (toLocaleUpperCase with SYSTEM_LANG)',
        heading.toLocaleUpperCase('el') !== heading.toUpperCase() && shown.some((text) => text.includes(heading.toLocaleUpperCase('el'))), shown.slice(0, 6).join(' / '));
    term.dispose();
    sandbox.SYSTEM_LANG = 'no such language tag!';
    const fallback = await renderDirectory(80, 30, translate);
    check('an invalid SYSTEM_LANG still renders headings', fallback.term.buffer.active.getLine(3).translateToString(true).length > 0);
    fallback.term.dispose();
    sandbox.SYSTEM_LANG = 'en';
}

// 7. Keys, admin keys, status refresh, clicks, racing loads, dispose and errors.
async function harness(options = {}) {
    const real = new sandbox.Terminal({ cols: 80, rows: options.rows || 30 });
    const mouse = clickable(real);
    const calls = [];
    const dialed = [];
    const edited = [];
    const deleted = [];
    const data = options.data || payload;
    const api = options.api || (async (url, request) => {
        calls.push(((request && request.method) || 'GET') + ' ' + url);
        if (url === '/api/desktop/retronet/directory') return JSON.parse(JSON.stringify(data));
        if (url === '/api/desktop/retronet/status') return { status: { telehack: { state: 'offline' } } };
        throw new Error('unexpected ' + url);
    });
    const directory = Directory.create({
        term: mouse.term, t, api, announce() {},
        canEdit: options.canEdit || (() => true),
        onDial: (entry) => dialed.push(entry.id),
        onEdit: (entry) => edited.push(entry ? entry.id : null),
        onDelete: (entry) => deleted.push(entry.id)
    });
    const screen = async () => {
        await flush(real);
        const shown = [];
        for (let y = 0; y < real.rows; y += 1) shown.push(real.buffer.active.getLine(y).translateToString(true));
        return shown;
    };
    const rowOf = async (pattern) => (await screen()).findIndex((text) => pattern.test(text));
    return { real, directory, calls, dialed, edited, deleted, screen, rowOf, ...mouse };
}
const settle = () => new Promise((resolve) => setTimeout(resolve, 0));
const deferred = () => {
    let resolve;
    let reject;
    const promise = new Promise((ok, fail) => { resolve = ok; reject = fail; });
    return { promise, resolve, reject };
};

{
    const h = await harness({ rows: 12 });
    await h.directory.load();
    h.directory.handleData('\x1b[F');
    same('End selects the last entry', h.directory.selected().id, 'old');
    h.directory.handleData('\x1b[6~');
    same('PgDn stops at the last entry', h.directory.selected().id, 'old');
    h.directory.handleData('\x1b[H');
    same('Home selects the local shell', h.directory.selected().id, Directory.LOCAL_SHELL_ID);
    h.directory.handleData('\x1b[6~');
    const paged = NUMBERED_IDS.indexOf(h.directory.selected().id);
    check('PgDn moves by a page', paged > 1, String(paged));
    h.directory.handleData('\x1b[5~');
    same('PgUp moves back by a page', h.directory.selected().id, Directory.LOCAL_SHELL_ID);
    h.directory.handleData('\x1bOB');
    same('application cursor down', h.directory.selected().id, 'telehack');
    h.directory.handleData('\x1b[A');
    same('cursor up', h.directory.selected().id, Directory.LOCAL_SHELL_ID);
    h.directory.handleData('0');
    h.directory.handleData('5');
    same('two digits within a second jump to 05', h.directory.selected().id, 'zwj');
    h.directory.handleData('1');
    h.directory.handleData('2');
    same('an unknown two-digit number keeps the first digit', h.directory.selected().id, 'telehack');
    h.directory.handleData('3');
    same('the next digit starts a new number', h.directory.selected().id, 'cjk');
    h.directory.dispose();
    h.real.dispose();
}
{
    const h = await harness();
    await h.directory.load();
    check('a fresh directory does not refresh status', !h.calls.includes('POST /api/desktop/retronet/status'));
    h.directory.handleData('R');
    await settle();
    check('R posts a status refresh', h.calls.includes('POST /api/desktop/retronet/status'), h.calls.join(', '));
    const row = (await h.screen()).find((text) => /^[> ]01 /.test(text)) || '';
    same('refreshed status is drawn', row.slice(4, 7), '[ ]');
    h.directory.dispose();
    h.real.dispose();
}
{
    const h = await harness({ data: { ...payload, stale: true } });
    await h.directory.load();
    await settle();
    check('a stale directory refreshes status after loading', h.calls.join(', ') === 'GET /api/desktop/retronet/directory, POST /api/desktop/retronet/status', h.calls.join(', '));
    h.directory.dispose();
    h.real.dispose();
}
{
    const h = await harness();
    await h.directory.load();
    h.directory.handleData('2');
    await settle();
    h.directory.handleData('e');
    h.directory.handleData('\x1b[3~');
    h.directory.handleData('n');
    check('admin keys act on own entries', h.edited.join(',') === 'ascii,' && h.deleted.join(',') === 'ascii', h.edited + ' / ' + h.deleted);
    check('admin key help is shown', (await h.screen()).some((text) => text.includes('N: new entry')));
    h.directory.handleData('1');
    same('a non-digit key ends the number being typed', h.directory.selected().id, 'telehack');
    h.directory.handleData('E');
    h.directory.handleData('\x1b[3~');
    check('catalog entries cannot be edited or deleted', h.edited.length === 2 && h.deleted.length === 1, h.edited + ' / ' + h.deleted);
    check('the not-own notice is shown', (await h.screen())[0].includes(t('desktop.terminal_retronet_not_own')));
    h.directory.handleData('\x1b[B');
    check('moving clears the not-own notice', !(await h.screen())[0].includes(t('desktop.terminal_retronet_not_own')));
    h.directory.dispose();
    h.real.dispose();
}
for (const [label, options] of [['can_edit false', { data: { ...payload, can_edit: false } }], ['read-only desktop', { canEdit: () => false }]]) {
    const h = await harness(options);
    await h.directory.load();
    h.directory.handleData('2');
    h.directory.handleData('n');
    h.directory.handleData('e');
    h.directory.handleData('\x1b[3~');
    check(label + ': n/e/Del are refused', h.edited.length === 0 && h.deleted.length === 0, h.edited + ' / ' + h.deleted);
    check(label + ': admin key help is hidden', !(await h.screen()).some((text) => text.includes('N: new entry')));
    h.directory.dispose();
    h.real.dispose();
}
{
    const h = await harness();
    await h.directory.load();
    const heading = await h.rowOf(/^ CLASSICS/);
    check('the classics heading is on screen', heading > 0, String(heading));
    h.click(heading);
    same('a click on a heading selects nothing', h.directory.selected().id, Directory.LOCAL_SHELL_ID);
    h.click(0);
    h.click(1);
    same('a click on the header selects nothing', h.directory.selected().id, Directory.LOCAL_SHELL_ID);
    h.click(-2);
    h.click(h.real.rows + 2);
    same('clicks outside the screen are ignored', h.directory.selected().id, Directory.LOCAL_SHELL_ID);
    const footerRule = (await h.screen()).findIndex((text, index) => index > 2 && text.startsWith('\u2500'));
    h.click(footerRule + 1);
    same('a click on the key help selects nothing', h.directory.selected().id, Directory.LOCAL_SHELL_ID);
    const telehack = await h.rowOf(/^[> ]01 /);
    h.click(telehack);
    same('a click selects the entry', h.directory.selected().id, 'telehack');
    check('a single click does not dial', h.dialed.length === 0);
    h.click(telehack, 'dblclick');
    same('a double-click dials the entry', h.dialed.join(','), 'telehack');
    h.directory.dispose();
    h.real.dispose();
}
{
    const h = await harness();
    await h.directory.load();
    const cjk = await h.rowOf(/^[> ]03 /);
    h.click(cjk);
    h.click(cjk);
    same('a double tap dials the entry', h.dialed.join(','), 'cjk');
    h.directory.dispose();
    h.real.dispose();
}
{
    const pending = [];
    const h = await harness({ api: () => { const d = deferred(); pending.push(d); return d.promise; } });
    const first = h.directory.load();
    const second = h.directory.load();
    await settle();
    same('two loads fetch twice', pending.length, 2);
    pending[1].resolve({ entries: [payload.entries[8]], status: {}, stale: false, can_edit: false });
    same('the newer load wins', await second, true);
    pending[0].resolve(JSON.parse(JSON.stringify(payload)));
    same('the older load is ignored', await first, false);
    same('entries come from the newer load', h.directory.entries().map((entry) => entry.id).join(','), 'old');
    h.directory.dispose();
    h.real.dispose();
}
{
    const pending = [];
    const h = await harness({ api: () => { const d = deferred(); pending.push(d); return d.promise; } });
    const loading = h.directory.load();
    await settle();
    h.directory.dispose();
    const before = h.writes();
    pending[0].resolve(JSON.parse(JSON.stringify(payload)));
    same('a load finishing after dispose resolves false', await loading, false);
    same('nothing is drawn after dispose', h.writes(), before);
    h.directory.handleData('\r');
    same('keys after dispose are ignored', h.dialed.length, 0);
    h.real.dispose();
}
{
    let attempts = 0;
    const h = await harness({
        api: async (url) => {
            attempts += 1;
            if (attempts === 1) {
                const err = new Error('forbidden');
                err.status = 403;
                throw err;
            }
            return url === '/api/desktop/retronet/directory' ? JSON.parse(JSON.stringify(payload)) : { status: {} };
        }
    });
    let rejected = null;
    try {
        await h.directory.load();
    } catch (err) {
        rejected = err;
    }
    same('a 403 rejects load() with the error', rejected && rejected.status, 403);
    check('the error view is drawn', (await h.screen())[0].includes(t('desktop.terminal_retronet_load_failed')));
    h.directory.handleData('r');
    await settle();
    await settle();
    same('R after a failed load loads again', h.directory.entries().length, payload.entries.length);
    h.directory.dispose();
    h.real.dispose();
}

if (failures) {
    console.log(failures + ' check(s) failed');
    process.exit(1);
}
console.log('terminal retronet directory ok');
