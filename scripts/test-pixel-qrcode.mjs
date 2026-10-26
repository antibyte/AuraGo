import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import path from 'node:path';
import { installFakeDom, makeEl, loadPixelApp, englishT } from './pixel-test-dom.mjs';
import { repoRoot } from './pixel-test-util.mjs';

installFakeDom();
const Pixel = loadPixelApp();
if (!Pixel.listModules().some(module => module.id === 'qrcode'))
    vm.runInThisContext(fs.readFileSync(path.join(repoRoot, 'ui/js/desktop/apps/pixel-qrcode.js'), 'utf8'));
const runtimes = [];
Pixel.registerModule('qr-test-runtime', runtime => runtimes.push(runtime));
let options, closed, closes = 0, loader = async () => {};
window.AuraDesktopModules = { loadAppAssets: key => { assert.equal(key, 'qr'); return loader(); } };
window.AuraQRDialog = { open(value) {
    options = value;
    return { closed: new Promise(resolve => { closed = resolve; }), close() { closes++; closed(); } };
} };
let decoded = 0, released = 0;
globalThis.createImageBitmap = async () => { decoded++; return { width: 21, height: 21, close() { released++; } }; };

function windowWithDocument(id, readonly = false) {
    window.PixelApp.render(makeEl('div'), id, { t: englishT(), api: async () => ({}), iconMarkup: () => '', notify: () => {}, readonly });
    const runtime = runtimes.at(-1);
    runtime.newBlankCanvas(200, 160);
    return runtime;
}
const runtime = windowWithDocument('qr-test');
const first = runtime.layers[0], historyCount = runtime.history.length;
runtime.selection = { type: 'rect', x: 2, y: 3, w: 5, h: 6 };
const selection = runtime.selection;
const open = runtime.insertQR();
await Promise.resolve(); await Promise.resolve();
assert.equal(options.windowId, 'qr-test');
assert.equal(options.insertion.unit, 'px');
await options.onInsert({ file: {}, insertWidth: 60, insertHeight: 60, x: 20, y: 30 });
assert.equal(runtime.layers.length, 2);
assert.equal(runtime.layers[0], first, 'prior layer identity is preserved');
assert.equal(runtime.layers[1].sticker, undefined, 'QR is an ordinary raster layer');
assert.equal(runtime.selection, selection, 'insertion preserves the selection');
assert.equal(runtime.history.length, historyCount + 1, 'one insertion produces one undo step');
assert.equal(runtime.layers[1].name, 'QR code');
closed(); await open;
runtime.undo(); assert.equal(runtime.layers.length, 1);
runtime.redo(); assert.equal(runtime.layers.length, 2);

const staleOpen = runtime.insertQR();
await Promise.resolve(); await Promise.resolve();
const staleOptions = options;
runtime.newBlankCanvas(200, 160);
await assert.rejects(staleOptions.onInsert({ file: {}, insertWidth: 60, insertHeight: 60, x: 0, y: 0 }), /changed|again/i);
assert.equal(runtime.layers.length, 1, 'late result cannot touch a new document');
closed(); await staleOpen;

const boundsOpen = runtime.insertQR();
await Promise.resolve(); await Promise.resolve();
await assert.rejects(options.onInsert({ file: {}, insertWidth: 250, insertHeight: 60, x: 0, y: 0 }), /fit|position|canvas/i);
assert.equal(runtime.layers.length, 1);
closed(); await boundsOpen;

let finishLoad;
loader = () => new Promise(resolve => { finishLoad = resolve; });
const delayed = runtime.insertQR();
const oldOptions = options;
runtime.newBlankCanvas(300, 220); finishLoad(); await delayed;
assert.equal(options, oldOptions, 'changing a document during lazy load never opens a stale dialog');
loader = async () => {};
const disposeOpen = runtime.insertQR();
await Promise.resolve(); await Promise.resolve();
const signal = options.signal;
window.PixelApp.dispose('qr-test'); await disposeOpen;
assert.equal(signal.aborted, true); assert.equal(closes, 1);
assert.equal(decoded, released, 'all decoded bitmaps are released');
const readOnly = windowWithDocument('qr-readonly', true), before = options;
await readOnly.insertQR(); assert.equal(options, before);
window.PixelApp.dispose('qr-readonly');
console.log('test-pixel-qrcode: native raster insertion, undo/redo, revision fences, bounds, readonly and disposal passed');
