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

if (failures) {
    console.log(failures + ' check(s) failed');
    process.exit(1);
}
console.log('terminal retronet directory ok');
