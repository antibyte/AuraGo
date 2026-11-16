import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const sources = [
    '../ui/js/desktop/shared/calculator-expression.js',
    '../ui/js/desktop/apps/calculator.js',
    '../ui/js/desktop/core/settings-catalog-runtime.js',
    '../ui/js/desktop/core/file-associations-runtime.js',
    '../ui/js/desktop/core/search-results-runtime.js'
];
const window = {};
const context = vm.createContext({ window, URLSearchParams, setTimeout: () => 1, clearTimeout() {} });
for (const source of sources) {
    const path = fileURLToPath(new URL(source, import.meta.url));
    vm.runInContext(await readFile(path, 'utf8'), context, { filename: source });
}

const calculator = window.AuraCalculatorExpression;
assert.equal(calculator.evaluate('2 + 3 * 4'), 14);
assert.equal(calculator.evaluate('sqrt(81) + 1'), 10);
assert.throws(() => calculator.evaluate('1 / 0'), /Invalid expression/);

function fakeElement() {
    const classes = new Set();
    return {
        dataset: {},
        style: { setProperty() {}, getPropertyValue() { return ''; } },
        classList: {
            add(name) { classes.add(name); },
            remove(name) { classes.delete(name); },
            toggle(name, force) {
                const next = force === undefined ? !classes.has(name) : !!force;
                if (next) classes.add(name); else classes.delete(name);
                return next;
            },
            contains(name) { return classes.has(name); }
        },
        listeners: {},
        addEventListener(type, listener) { this.listeners[type] = listener; },
        querySelector() { return fakeElement(); },
        querySelectorAll() { return []; },
        closest() { return null; },
        setAttribute() {},
        focus() {},
        textContent: '',
        innerHTML: '',
        scrollWidth: 0,
        clientWidth: 100,
        hidden: false
    };
}
const calculatorElements = new Map();
const calculatorKeys = ['C', '0', '1', '2', '4', '5', '6', '7', '8', '9', '/', 'MOD', '='].map(key => {
    const element = fakeElement();
    element.dataset.key = key;
    return element;
});
const calculatorModes = ['standard', 'scientific', 'programmer'].map(mode => {
    const element = fakeElement();
    element.dataset.mode = mode;
    return element;
});
const calculatorHost = {
    innerHTML: '',
    querySelector(selector) {
        if (!calculatorElements.has(selector)) calculatorElements.set(selector, fakeElement());
        return calculatorElements.get(selector);
    },
    querySelectorAll(selector) {
        if (selector === '[data-mode]') return calculatorModes;
        if (selector === '[data-key]') return calculatorKeys;
        return [];
    }
};
window.CalculatorApp.render(calculatorHost, 'calc-search-test', {
    expression: '2 + 3 * 4',
    esc: value => String(value),
    t: key => key,
    registerWindowCleanup() {},
    showContextMenu() {},
    wireContextMenuBoundary() {}
});
assert.equal(calculatorElements.get('[data-expression]').textContent, '2 + 3 * 4');
assert.equal(calculatorElements.get('[data-result]').textContent, '14');
calculatorModes[2].listeners.click();
assert.equal(calculatorModes[2].classList.contains('active'), true);
assert.equal(window.CalculatorApp.openExpression('calc-search-test', '10 + 10'), true);
assert.equal(calculatorElements.get('[data-expression]').textContent, '10 + 10');
assert.equal(calculatorElements.get('[data-result]').textContent, '20', 'search arithmetic remains decimal after leaving programmer mode');
assert.equal(calculatorModes[0].classList.contains('active'), true, 'search arithmetic selects a standard mode from programmer mode');
assert.equal(calculatorModes[2].classList.contains('active'), false);
assert.equal(window.CalculatorApp.openExpression('calc-search-test', 'not a safe expression'), false);

const pressCalculatorKey = key => {
    const button = calculatorKeys.find(element => element.dataset.key === key);
    assert.equal(typeof button?.listeners.click, 'function', `programmer key ${key} should be wired`);
    button.listeners.click();
};
calculatorModes[2].listeners.click();
pressCalculatorKey('C');
for (const key of ['9', '/', '2', '=']) pressCalculatorKey(key);
assert.equal(calculatorElements.get('[data-result]').textContent, '4', 'programmer division should retain integer division');
pressCalculatorKey('C');
for (const key of ['1', '/', '0', '=']) pressCalculatorKey(key);
assert.equal(calculatorElements.get('[data-result]').textContent, 'desktop.calc_invalid_expression', 'programmer division by zero should show the normal invalid-expression state');
assert.equal(calculatorElements.get('[data-result]').classList.contains('error'), true);
pressCalculatorKey('C');
for (const key of ['1', 'MOD', '0', '=']) pressCalculatorKey(key);
assert.equal(calculatorElements.get('[data-result]').textContent, 'desktop.calc_invalid_expression', 'programmer MOD by zero should use the same divisor guard');
assert.equal(calculatorElements.get('[data-result]').classList.contains('error'), true);

const settingsContext = {
    state: { bootstrap: { providers: [], workspace: { root: 'Home' } }, iconThemeManifests: {} },
    settingValue: () => 'papirus',
    currentDesktopTheme: () => ({ schemeSetting: 'appearance.fruity_mode', accent: 'user' }),
    desktopThemes: () => [],
    t: key => key,
    openSetting: (category, settingKey) => ({ category, settingKey })
};
const sections = window.AuraDesktopSettingsCatalog.sections(settingsContext);
assert.equal(sections.length, 8, 'catalog should keep the eight Settings categories');
assert.ok(sections.find(section => section.id === 'appearance').items.some(item => item.key === 'accessibility.text_size'));
assert.ok(sections.find(section => section.id === 'appearance').items.some(item => item.key === 'accessibility.high_contrast'));
assert.ok(sections.find(section => section.id === 'files').items.some(item => item.key === 'files.default_apps'));
const settingRows = window.AuraDesktopSettingsCatalog.list('text size', settingsContext);
assert.equal(settingRows.length, 1);
assert.equal(settingRows[0].settingKey, 'accessibility.text_size');
assert.deepEqual(settingRows[0].open(), { category: 'appearance', settingKey: 'accessibility.text_size' });

const local = window.AuraDesktopSearch.local('open trash', {
    apps: [{ id: 'trash-viewer', type: 'app', title: 'Trash Viewer', open() {} }],
    actions: [{ id: 'open-trash', type: 'action', title: 'Open Trash', open() {} }],
    t: key => key
});
assert.equal(local.length, 1);
assert.equal(local[0].id, 'open-trash');
const calculatorResult = window.AuraDesktopSearch.local('2 + 2', { t: key => key, openCalculator() {} });
assert.equal(calculatorResult.length, 1);
assert.equal(calculatorResult[0].value, '4');

let requestedURL = '';
let openedFile = '';
const provider = window.AuraDesktopSearch.createProvider({
    api: async (url, options) => {
        requestedURL = url;
        assert.equal(options.signal.aborted, false);
        return {
            files: [{ name: 'photo.png', path: 'Pictures/photo.png' }],
            has_more: true,
            truncated: false,
            next_offset: 1
        };
    },
    openFile: path => { openedFile = path; }
});
const page = await provider.files('photo', {
    signal: new AbortController().signal,
    filters: { type: 'image', minSize: 10 },
    limit: 1,
    offset: 0
});
assert.match(requestedURL, /type=image/);
assert.match(requestedURL, /min_size=10/);
assert.equal(page.hasMore, true);
assert.equal(page.items[0].type, 'file');
page.items[0].open();
assert.equal(openedFile, 'Pictures/photo.png');

const shortSearchRequests = [];
const filteredProvider = window.AuraDesktopSearch.createProvider({
    api: async url => {
        shortSearchRequests.push(url);
        return { files: [], has_more: false, truncated: false, next_offset: 0 };
    }
});
const emptyUnfiltered = await filteredProvider.files('', { filters: { type: 'all' } });
assert.equal(emptyUnfiltered.items.length, 0);
assert.equal(shortSearchRequests.length, 0, 'short unfiltered queries avoid the recursive endpoint');
await filteredProvider.files('', { filters: { type: 'image' } });
await filteredProvider.files('x', { filters: { type: 'all', minSize: 0 } });
assert.equal(shortSearchRequests.length, 2, 'active file filters permit empty and one-character queries');
assert.match(shortSearchRequests[0], /query=&.*type=image/);
assert.match(shortSearchRequests[1], /query=x&.*min_size=0/);

const associations = window.AuraDesktopFileAssociations;
const candidateApps = ['bookshelf', 'writer', 'sheets', 'notes', 'editor', 'code-studio', 'viewer', 'audio-player', 'music-player', 'gallery'].map(id => ({ id }));
const extensionOf = file => String(file.name || file.path || '').split(/[\\/]/).pop().split('.').pop().toLowerCase();
const actualOpenWithHelpers = {
    allApps: () => candidateApps,
    isWriterFile: file => ['docx', 'html', 'htm'].includes(extensionOf(file)),
    isSheetsFile: file => ['xlsx', 'xlsm', 'csv'].includes(extensionOf(file)),
    isViewerFile: file => ['md', 'pdf', 'docx', 'xlsx', 'xlsm', 'csv'].includes(extensionOf(file)),
    isImageFile: name => /\.(png|jpg|jpeg|gif|svg|webp|bmp|ico|tiff|tif|avif)$/i.test(name),
    isMediaFile: name => /\.(mp4|mkv|avi|mov|webm|mp3|wav|flac|ogg|m4a|opus)$/i.test(name),
    isArchiveFile: name => /\.(zip|7z|rar|tar|gz)$/i.test(name)
};
for (const [name, expected] of [
    ['report.docx', ['bookshelf', 'writer', 'viewer']],
    ['sheet.xlsx', ['sheets', 'viewer']],
    ['readme.md', ['notes', 'editor', 'viewer']],
    ['notes.txt', ['bookshelf', 'editor', 'code-studio', 'viewer']]
]) {
    const actual = associations.resolveCandidates({ name, type: 'file' }, actualOpenWithHelpers).map(item => item.appId);
    assert.deepEqual(Array.from(actual), expected, `${name} candidate order should preserve its specialized editor`);
}
assert.deepEqual(Array.from(associations.resolveCandidates({ name: 'recording.aac', media_kind: 'audio' }, actualOpenWithHelpers).map(item => item.appId)), ['audio-player', 'music-player', 'gallery', 'viewer']);
const candidates = associations.resolveCandidates({ name: 'photo.png' }, {
    isImageFile: name => name.endsWith('.png'),
    isViewerFile: () => false,
    isMediaFile: () => false
});
assert.deepEqual(Array.from(candidates, item => item.appId), ['gallery', 'viewer', 'code-studio']);
const rows = associations.list({ '.png': 'removed-app', txt: 'viewer' }, [{ id: 'viewer', name: 'Viewer' }], () => candidates);
assert.equal(rows[0].extension, 'png');
assert.equal(rows[0].available, false);
assert.equal(rows[0].appId, 'removed-app');
assert.deepEqual(Array.from(rows[0].candidates, item => item.appId), ['viewer']);

process.stdout.write('Desktop search helpers passed.\n');
