import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { readFile, mkdir } from 'node:fs/promises';
import { createRequire } from 'node:module';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const scripts = ['shared/sticker-catalog', 'shared/sticker-library', ...['state', 'lib-core', 'lib-fx', 'lib-gen', 'lib-cutout', 'lib-inpaint', 'core', 'view', 'canvas', 'tools', 'actions', 'filters', 'events', 'stickers', 'layers', 'selection', 'brush', 'tools-pro', 'transform', 'generators', 'cutout', 'smart', 'commands', 'ora'].map(name => 'apps/pixel-' + name), 'apps/pixel'];
const server = createServer(async (req, res) => {
    try {
        const name = new URL(req.url, 'http://localhost').pathname;
        if (name === '/') { res.setHeader('Content-Type', 'text/html'); res.end('<!doctype html><meta charset="utf-8"><body class="desktop-body"><div id="host" style="position:absolute;inset:0"></div><div id="second" hidden></div>'); return; }
        const file = path.resolve(root, 'ui', '.' + name);
        if (!file.startsWith(path.join(root, 'ui') + path.sep)) throw Error('path');
        res.setHeader('Content-Type', name.endsWith('.css') ? 'text/css' : name.endsWith('.png') ? 'image/png' : 'text/javascript');
        res.end(await readFile(file));
    } catch { res.statusCode = 404; res.end(); }
});
await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
const browser = await chromium.launch({ headless: true, ...(process.env.PLAYWRIGHT_CHANNEL ? { channel: process.env.PLAYWRIGHT_CHANNEL } : {}) });
const origin = 'http://127.0.0.1:' + server.address().port;
const report = path.join(root, 'reports/stickers'); await mkdir(report, { recursive: true });
try {
    for (const theme of ['standard', 'fruity']) for (const width of [1280, 520]) {
        const page = await browser.newPage({ viewport: { width, height: 900 } }), errors = [], originals = [];
        page.on('pageerror', error => errors.push(error.message));
        page.on('request', req => { if (/\/img\/stickers\/[^/]+\.png/.test(req.url())) originals.push(req.url()); });
        const locale = theme === 'fruity' ? 'de' : 'en';
        const strings = JSON.parse(await readFile(path.join(root, 'ui/lang/desktop/' + locale + '.json'), 'utf8'));
        await page.goto(origin);
        for (const css of ['/shared-variables.css', '/shared-utilities.css', '/shared-components.css', '/css/desktop-shell.bundle.css', '/css/desktop-app-common.css', '/css/pixel.css', '/css/desktop-stickers.css', '/css/pixel-stickers.css', '/css/pixel-layers.css', '/css/pixel-filters.css', '/css/pixel-generators.css', '/css/pixel-cutout.css', '/css/pixel-selection.css', '/css/pixel-tools-pro.css', '/css/pixel-smart.css']) await page.addStyleTag({ url: origin + css });
        for (const script of scripts) await page.addScriptTag({ url: origin + '/js/desktop/' + script + '.js' });
        await page.evaluate(({ theme, strings }) => {
            document.body.dataset.theme = theme; document.body.dataset.fruityMode = 'light';
            window.AuraLazyAssets = { versionedURL: url => url + '?v=sticker-test' };
            window.testRuntimes = []; window.testNotices = []; window.testSaved = [];
            const install = AuraPixelApp.installStickers;
            AuraPixelApp.installStickers = r => { install(r); testRuntimes.push(r); };
            window.testContext = { t: key => strings[key] || key, esc: value => String(value).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c])),
                notify: value => testNotices.push(value), api: async (url, options) => { if (url === '/api/pixel/save') { testSaved.push(JSON.parse(options.body)); return { path: 'Pictures/sticker-test.png', version: 'v2' }; } return {}; } };
            PixelApp.render(document.querySelector('#host'), 'one', testContext); window.r = testRuntimes[0];
        }, { theme, strings });
        await page.locator('[data-panel="stickers"]').click();
        assert.equal(await page.locator('[data-sticker-id]').count(), 100);
        assert.equal(originals.length, 0, 'picker must only load thumbnails');
        assert(await page.locator('[data-sticker-id]').first().isDisabled(), 'no image');
        await page.evaluate(() => r.newBlankCanvas(1280, 720));
        const search = page.locator('.sticker-picker input[type=search]');
        await search.fill(strings['stickers.name.nature-cactus']);
        assert.equal(await page.locator('[data-sticker-id]').count(), 1);
        await search.fill('no-such-sticker-123'); assert.equal(await page.locator('[data-sticker-id]').count(), 0);
        await search.fill('');
        const filters = page.locator('.sticker-picker select');
        await filters.nth(0).selectOption('animals'); assert.equal(await page.locator('[data-sticker-id]').count(), 10);
        await filters.nth(1).selectOption('clay'); assert.equal(await page.locator('[data-sticker-id]').count(), 1);
        await filters.nth(0).selectOption(''); await filters.nth(1).selectOption('');
        await page.route('**/img/stickers/thumbs/animals-cat.png?*', route => route.fulfill({ status: 404, body: '' }));
        await search.fill(strings['stickers.name.animals-cat']);
        await page.locator('[data-sticker-id="animals-cat"].is-unavailable').waitFor();
        assert((await page.locator('[data-sticker-id="animals-cat"]').getAttribute('aria-label')).includes(strings['stickers.failed']), 'missing thumbnail has accessible feedback');
        await page.unroute('**/img/stickers/thumbs/animals-cat.png?*'); await search.fill('');
        const first = page.locator('[data-sticker-id="emotion-joy"]');
        await first.focus(); await page.keyboard.press('Enter');
        await page.waitForFunction(() => r.layers.length === 2);
        assert(originals.every(url => url.includes('?v=sticker-test')), 'versioned original requests');
        assert.equal(await page.evaluate(() => r.layers[1].sticker.id), 'emotion-joy');
        await page.evaluate(() => {
            const check = (ok, message) => { if (!ok) throw Error(message); };
            const layer = () => r.layers[r.activeLayerIdx], pixels = canvas => canvas.getContext('2d').getImageData(0, 0, canvas.width, canvas.height).data;
            const field = (key, value) => { const input = r.host.querySelector('[data-sticker-field="' + key + '"]'); input.value = value; input.dispatchEvent(new Event('change', { bubbles: true })); };
            const matrix = layer().sticker.matrix.slice(), original = pixels(layer().canvas);
            check(Math.abs(matrix[0] * 1024 - 172.8) < .001, '24% initial scale');
            field('size', 20); field('size', 172.8);
            const restored = pixels(layer().canvas); check(original.every((v, i) => v === restored[i]), 'rescale must use the unchanged original: ' + JSON.stringify({ before: matrix, after: layer().sticker.matrix, different: original.reduce((n,v,i) => n + (v !== restored[i] ? 1 : 0), 0), first: original.findIndex((v,i) => v !== restored[i]) }));
            field('angle', 35); const rotated = layer().sticker.matrix.slice(); r.duplicateLayer();
            check(layer().sticker.id === 'emotion-joy' && layer().sticker.matrix.every((v,i) => v === rotated[i]), 'duplicate metadata');
            layer().sticker.matrix[4] += 9; check(r.layers[1].sticker.matrix[4] === rotated[4], 'duplicate matrix must not alias');
            r.undo(); check(r.layers.length === 2 && layer().sticker, 'undo duplicate'); r.redo(); check(r.layers.length === 3 && layer().sticker, 'redo duplicate');
            r.deleteLayer(); r.rotateCanvas(90);
            const afterRotate = new DOMMatrix([0, 1, -1, 0, 720, 0]).multiply(new DOMMatrix(rotated));
            check(Math.abs(layer().sticker.matrix[4] - afterRotate.e) < .001 && r.imgWidth === 720, 'whole-image rotation');
            r.flipCanvas(true); const afterFlip = new DOMMatrix([-1, 0, 0, 1, 720, 0]).multiply(afterRotate);
            check(Math.abs(layer().sticker.matrix[4] - afterFlip.e) < .001, 'whole-image flip');
            r.resizeImage(); const resizeWidth = document.querySelector('[data-resize-w]');
            resizeWidth.value = 360; resizeWidth.dispatchEvent(new Event('input')); resizeWidth.closest('form').dispatchEvent(new Event('submit', { cancelable: true }));
            check(Math.abs(layer().sticker.matrix[0] - afterFlip.a / 2) < .001, 'whole-image resize');
            const beforeCrop = layer().sticker.matrix.slice(); r.startCrop();
            const scaleX = parseFloat(r.canvas.style.width) / r.imgWidth, scaleY = parseFloat(r.canvas.style.height) / r.imgHeight;
            Object.assign(r.cropState, { startX: 10 * scaleX, startY: 20 * scaleY, endX: 350 * scaleX, endY: 620 * scaleY }); r.applyCrop();
            check(Math.abs(layer().sticker.matrix[4] - beforeCrop[4] + 10) < .001, 'crop matrix');
            const before = pixels(layer().canvas); r.setActiveTool('brush');
            r.onCanvasMouseDown({ button: 0, clientX: 30, clientY: 30 }); r.selectFilter('grayscale'); r.applyAdjustments(); r.toggleLayerMask(r.activeLayerIdx);
            r.selection = { x: 0, y: 0, w: 50, h: 50 }; r.deleteSelection(); r.deselect();
            const after = pixels(layer().canvas); check(before.every((v,i) => v === after[i]) && layer().sticker && !layer().maskCanvas, 'pixel editing requires rasterize: ' + JSON.stringify({ object: !!layer().sticker, mask: !!layer().maskCanvas, different: before.reduce((n,v,i) => n + (v !== after[i] ? 1 : 0), 0), notices: testNotices }));
            r.rasterizeSticker(); check(!layer().sticker, 'explicit rasterization');
            const background = pixels(r.layers[0].canvas), raster = pixels(layer().canvas);
            r.showPanel('adjust'); r.adjustments.brightness = 40; r.applyAdjustmentsPreview();
            const adjusted = pixels(layer().canvas), backgroundAfter = pixels(r.layers[0].canvas);
            check(adjusted.some((v,i) => v !== raster[i]) && background.every((v,i) => v === backgroundAfter[i]), 'adjust rasterized sticker without changing background');
            check(adjusted.every((v,i) => i % 4 !== 3 || v === raster[i]), 'adjustments preserve transparent silhouette');
            r.resetAdjustments(); const reset = pixels(layer().canvas);
            check(raster.every((v,i) => Math.abs(v - reset[i]) <= (i % 4 === 3 ? 0 : 1)), 'reset preserves raster pixels within 8-bit canvas rounding');
            r.adjustments.brightness = 40; r.applyAdjustments(); r.undo(); check(!layer().sticker, 'undo adjustment keeps raster layer');
            r.undo(); check(layer().sticker, 'undo rasterization'); r.showPanel('stickers');
            r.mergeDown(); check(r.layers.length === 1 && !layer().sticker, 'merge rasterizes'); r.undo(); check(layer().sticker, 'undo merge restores object');
            const mask = document.createElement('canvas'); mask.width = r.imgWidth; mask.height = r.imgHeight;
            const maskCtx = mask.getContext('2d'); maskCtx.fillStyle = '#fff'; maskCtx.fillRect(0, 0, mask.width / 2, mask.height);
            r.layers[0].maskCanvas = mask; r.layers[0].opacity = .4; layer().opacity = .63; r.compositeLayers(); window.previewPixels = pixels(r.canvas);
            const composed = document.createElement('canvas'); composed.width = r.imgWidth; composed.height = r.imgHeight;
            r.renderComposition(composed.getContext('2d')); const composedPixels = pixels(composed); check(previewPixels.every((v,i) => v === composedPixels[i]), 'composition matches preview');
        });
        await page.evaluate(async () => {
            await r.saveFile('Pictures/sticker-test.png');
            const saved = new Image(); saved.src = testSaved.at(-1).data; await saved.decode();
            const c = document.createElement('canvas'); c.width = r.imgWidth; c.height = r.imgHeight; c.getContext('2d').drawImage(saved, 0, 0);
            const pixels = c.getContext('2d').getImageData(0, 0, c.width, c.height).data;
            if (!previewPixels.every((v,i) => v === pixels[i]) || !r.layers[1].sticker) throw Error('save differs or flattens live document');
        });
        const downloadPromise = page.waitForEvent('download'); await page.evaluate(() => r.exportWithFormat('png', 1)); const download = await downloadPromise;
        const exportPath = path.join(report, 'pixel-export-' + theme + '-' + width + '.png'); await download.saveAs(exportPath);
        await page.evaluate(async data => {
            const image = new Image(); image.src = data; await image.decode();
            const canvas = document.createElement('canvas'); canvas.width = r.imgWidth; canvas.height = r.imgHeight;
            canvas.getContext('2d').drawImage(image, 0, 0); const pixels = canvas.getContext('2d').getImageData(0, 0, canvas.width, canvas.height).data;
            if (!previewPixels.every((v,i) => v === pixels[i])) throw Error('export pixels differ from preview');
        }, 'data:image/png;base64,' + (await readFile(exportPath)).toString('base64'));
        assert(await page.evaluate(() => !!r.layers[1].sticker), 'export keeps editable sticker');
        await page.evaluate(() => { r.newBlankCanvas(1280, 720); });
        await first.click(); await page.waitForFunction(() => r.layers.length === 2);
        await page.screenshot({ path: path.join(report, 'pixel-' + theme + '-' + width + '.png') });
        const handles = page.locator('.pixel-sticker-frame polygon'), bounds = await handles.boundingBox();
        if (bounds && bounds.width > 4) {
            const before = await page.evaluate(() => r.layers[1].sticker.matrix[4]);
            await page.mouse.move(bounds.x + bounds.width / 2, bounds.y + bounds.height / 2); await page.mouse.down(); await page.mouse.move(bounds.x + bounds.width / 2 + 15, bounds.y + bounds.height / 2 + 10); await page.mouse.up();
            assert.notEqual(await page.evaluate(() => r.layers[1].sticker.matrix[4]), before, 'pointer move');
            const sizeBefore = await page.evaluate(() => Math.hypot(...r.layers[1].sticker.matrix.slice(0,2)));
            const corner = await page.locator('.pixel-sticker-frame circle').first().boundingBox();
            await page.mouse.move(corner.x + corner.width / 2, corner.y + corner.height / 2); await page.mouse.down(); await page.mouse.move(corner.x - 15, corner.y - 15); await page.mouse.up();
            assert.notEqual(await page.evaluate(() => Math.hypot(...r.layers[1].sticker.matrix.slice(0,2))), sizeBefore, 'scale handle');
            const rotation = await page.locator('[data-sticker-handle="rotate"]').boundingBox();
            await page.mouse.move(rotation.x + rotation.width / 2, rotation.y + rotation.height / 2); await page.mouse.down(); await page.mouse.move(rotation.x + 24, rotation.y + 15); await page.mouse.up();
            assert.notEqual(await page.evaluate(() => r.layers[1].sticker.matrix[1]), 0, 'rotation handle');
            const matrixBefore = await page.evaluate(() => r.layers[1].sticker.matrix.slice());
            const polygon = await handles.boundingBox();
            await page.mouse.move(polygon.x + polygon.width / 2, polygon.y + polygon.height / 2); await page.mouse.down(); await page.mouse.move(polygon.x + polygon.width / 2 + 15, polygon.y + polygon.height / 2); await page.keyboard.press('Escape'); await page.mouse.up();
            assert.deepEqual(await page.evaluate(() => r.layers[1].sticker.matrix), matrixBefore, 'Escape cancels gesture');
        }
        await verifyStickerPicking(page);
        await page.evaluate(() => {
            PixelApp.render(document.querySelector('#second'), 'two', testContext); testRuntimes[1].newBlankCanvas(300, 300);
            if (testRuntimes[1].layers.length !== 1 || r.layers.length !== 2) throw Error('windows share layers');
            while (r.layers.length < 20) r.duplicateLayer();
        });
        await page.evaluate(() => r.addSticker('emotion-joy')); assert.equal(await page.evaluate(() => r.layers.length), 20);
        await page.evaluate(() => r.newBlankCanvas(600, 400));
        let release;
        const barrier = new Promise(resolve => { release = resolve; });
        await page.route('**/img/stickers/emotion-joy.png?*', async route => { await barrier; try { await route.continue(); } catch {} });
        const loading = page.waitForRequest('**/img/stickers/emotion-joy.png?*');
        await page.evaluate(() => { window.pendingSticker = r.addSticker('emotion-joy').catch(e => { if (e.name !== 'AbortError') throw e; }); });
        await loading;
        await page.evaluate(() => r.newBlankCanvas(320, 240)); release();
        await page.evaluate(() => pendingSticker); assert.equal(await page.evaluate(() => r.layers.length), 1, 'stale image load');
        await page.unroute('**/img/stickers/emotion-joy.png?*');
        await page.route('**/img/stickers/emotion-joy.png?*', route => route.fulfill({ status: 404, body: '' }));
        await first.click(); await page.waitForFunction(() => document.querySelector('.sticker-picker-status').textContent.includes('Could not') || document.querySelector('.sticker-picker-status').textContent.includes('nicht geladen'));
        assert.equal(await page.evaluate(() => r.layers.length), 1);
        await page.unroute('**/img/stickers/emotion-joy.png?*');
        let finish; const closingBarrier = new Promise(resolve => { finish = resolve; });
        await page.route('**/img/stickers/emotion-joy.png?*', async route => { await closingBarrier; try { await route.continue(); } catch {} });
        const closingLoad = page.waitForRequest('**/img/stickers/emotion-joy.png?*');
        await page.evaluate(() => { window.closingSticker = r.addSticker('emotion-joy').catch(e => { if (e.name !== 'AbortError') throw e; }); }); await closingLoad;
        await page.evaluate(() => { PixelApp.dispose('one'); PixelApp.dispose('two'); }); finish();
        await page.evaluate(() => closingSticker);
        assert.equal(await page.locator('.pixel-sticker-frame').count(), 0, 'dispose releases controls');
        assert.deepEqual(errors, []); await page.close();
        console.log('Pixel stickers passed: ' + theme + ', ' + width + 'px');
    }
} finally { await browser.close(); await new Promise(resolve => server.close(resolve)); }

async function verifyStickerPicking(page) {
    await page.evaluate(async () => {
        r.newBlankCanvas(1280, 720);
        for (const id of ['emotion-joy', 'animals-cat', 'nature-sun']) await r.addSticker(id);
        r.layers.slice(1).forEach((layer, i) => {
            const angle = (i - 1) * Math.PI / 6, a = Math.cos(angle) * 200 / 1024, b = Math.sin(angle) * 200 / 1024;
            layer.sticker.matrix = [a, b, -b, a, 200 + i * 440 - 512 * (a - b), 350 - 512 * (a + b)];
            r.repaintSticker(layer);
        });
        r.compositeLayers(); r.pushHistory('selection fixture');
    });
    const matrices = () => page.evaluate(() => r.layers.slice(1).map(layer => layer.sticker.matrix.slice()));
    const point = index => page.evaluate(index => {
        const m = r.layers[index].sticker.matrix, box = r.canvas.getBoundingClientRect();
        const x = Math.round(m[4] + 512 * (m[0] + m[2])), y = Math.round(m[5] + 512 * (m[1] + m[3]));
        if (!r.layers[index].canvas.getContext('2d').getImageData(x, y, 1, 1).data[3]) throw Error('fixture needs an opaque centre');
        return { x: box.left + x * box.width / r.imgWidth, y: box.top + y * box.height / r.imgHeight };
    }, index);
    for (const index of [1, 2, 3, 1]) {
        const before = await matrices(), start = await point(index);
        await page.mouse.move(start.x, start.y); await page.mouse.down();
        assert.equal(await page.evaluate(() => r.activeLayerIdx), index, 'canvas must select the clicked sticker, including older ones');
        assert.equal(await page.locator('.pixel-layer-item.active').getAttribute('data-layer-idx'), String(index));
        await page.mouse.move(start.x + 12, start.y + 8); await page.mouse.up();
        const after = await matrices();
        assert.notDeepEqual(after[index - 1], before[index - 1], 'clicked sticker must move');
        after.forEach((matrix, i) => { if (i !== index - 1) assert.deepEqual(matrix, before[i], 'other stickers must stay put'); });
        await page.evaluate(() => r.undo()); assert.deepEqual(await matrices(), before, 'undo selected sticker move');
        await page.evaluate(() => r.redo()); assert.deepEqual(await matrices(), after, 'redo selected sticker move');
    }
    await page.evaluate(() => {
        r.layers[3].sticker.matrix = r.layers[2].sticker.matrix.slice(); r.repaintSticker(r.layers[3]);
        r.activeLayerIdx = 0; r.compositeLayers(); r.refreshLayerPanel();
    });
    const overlap = await point(3);
    await page.mouse.click(overlap.x, overlap.y);
    assert.equal(await page.evaluate(() => r.activeLayerIdx), 3, 'top sticker selected from a raster layer');
    for (const property of ['visible', 'opacity', 'mask']) {
        await page.evaluate(property => {
            r.layers[3].visible = true; r.layers[3].opacity = 1;
            delete r.layers[3].maskCanvas;
            if (property === 'mask') {
                const mask = document.createElement('canvas'); mask.width = r.imgWidth; mask.height = r.imgHeight;
                r.layers[3].maskCanvas = mask;
            } else r.layers[3][property] = property === 'visible' ? false : 0;
            r.activeLayerIdx = 1; r.compositeLayers(); r.refreshLayerPanel();
        }, property);
        await page.mouse.click(overlap.x, overlap.y);
        assert.equal(await page.evaluate(() => r.activeLayerIdx), 2, property + ' must exclude invisible stickers from picking');
    }
    const transparent = await page.evaluate(() => {
        r.layers[3].opacity = 1; delete r.layers[3].maskCanvas;
        r.activeLayerIdx = 3; r.compositeLayers(); r.refreshLayerPanel();
        const lower = r.layers[2].canvas.getContext('2d').getImageData(0, 0, r.imgWidth, r.imgHeight).data;
        const upper = r.layers[3].canvas.getContext('2d').getImageData(0, 0, r.imgWidth, r.imgHeight).data;
        const m = r.layers[3].sticker.matrix, box = r.canvas.getBoundingClientRect();
        for (let y = Math.ceil(m[5] + 20); y < m[5] + 180; y++) for (let x = Math.ceil(m[4] + 20); x < m[4] + 180; x++) {
            const alpha = (y * r.imgWidth + x) * 4 + 3;
            if (lower[alpha] > 200 && upper[alpha] === 0) {
                const p = { x: box.left + x * box.width / r.imgWidth, y: box.top + y * box.height / r.imgHeight };
                const handle = document.elementFromPoint(p.x, p.y)?.dataset.stickerHandle;
                if (!handle || handle === 'move') return p;
            }
        }
        throw Error('fixture needs a transparent area over the lower sticker');
    });
    await page.mouse.click(transparent.x, transparent.y);
    assert.equal(await page.evaluate(() => r.activeLayerIdx), 2, 'active frame must allow picking through transparent sticker pixels');
    await page.evaluate(async () => { r.newBlankCanvas(1280, 720); await r.addSticker('emotion-joy'); });
}
