import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {readFile, mkdir} from 'node:fs/promises';
import {createRequire} from 'node:module';
import path from 'node:path';
import {fileURLToPath} from 'node:url';

const require = createRequire(import.meta.url);
const runtimeEnv = globalThis.process?.env || {};
const {chromium} = require(runtimeEnv.AURAGO_PLAYWRIGHT || globalThis.AURAGO_PLAYWRIGHT || 'playwright');
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const words = JSON.parse(await readFile(path.join(root, 'ui/lang/desktop/en.json'), 'utf8'));
const report = path.join(root, 'reports/chess/browser');
await mkdir(report, {recursive: true});
const types = {'.css': 'text/css', '.js': 'text/javascript', '.json': 'application/json', '.svg': 'image/svg+xml', '.glb': 'model/gltf-binary', '.png': 'image/png', '.wasm': 'application/wasm', '.woff2': 'font/woff2'};
const server = createServer(async (req, res) => {
    try {
        const name = new URL(req.url, 'http://localhost').pathname;
        if (name === '/') {
            res.setHeader('Content-Type', 'text/html');
            res.end('<!doctype html><meta charset="utf-8"><style>html,body{margin:0;width:100%;height:100%;overflow:hidden;background:#111823}#host{position:absolute;inset:0}</style><div id="host"></div>');
            return;
        }
        const file = path.resolve(root, 'ui', '.' + name);
        if (!file.startsWith(path.join(root, 'ui') + path.sep)) throw Error('invalid path');
        res.setHeader('Content-Type', types[path.extname(file)] || 'text/plain');
        res.end(await readFile(file));
    } catch {
        res.statusCode = 404;
        res.end();
    }
});
await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
const origin = `http://127.0.0.1:${server.address().port}`;
const browser = await chromium.launch({headless: true, ...((runtimeEnv.PLAYWRIGHT_CHANNEL || globalThis.PLAYWRIGHT_CHANNEL) ? {channel: runtimeEnv.PLAYWRIGHT_CHANNEL || globalThis.PLAYWRIGHT_CHANNEL} : {}), ...((runtimeEnv.AURAGO_CHROMIUM_EXECUTABLE || globalThis.AURAGO_CHROMIUM_EXECUTABLE) ? {executablePath: runtimeEnv.AURAGO_CHROMIUM_EXECUTABLE || globalThis.AURAGO_CHROMIUM_EXECUTABLE} : {})});
try {
    for (const width of [1600, 1100, 760]) {
        const page = await browser.newPage({viewport: {width, height: 620}});
        const errors = [];
        page.on('pageerror', e => errors.push(e.message));
        await page.route('**/api/desktop/chess/**', route => route.fulfill({status: 200, contentType: 'application/json', body: JSON.stringify({ok: true})}));
        await page.goto(origin);
        await page.addStyleTag({url: `${origin}/css/desktop-shell.bundle.css`});
        await page.addStyleTag({url: `${origin}/css/cm-chessboard.css`});
        await page.addStyleTag({url: `${origin}/css/desktop-app-chess.css`});
        for (const file of ['vendor/three.min.js', 'vendor/GLTFLoader.min.js', 'vendor/OrbitControls.min.js', 'desktop/apps/chess-engine.js', 'desktop/apps/chess-agent.js', 'desktop/apps/chess-fx.js', 'desktop/apps/chess-3d.js', 'desktop/apps/chess.js']) {
            await page.addScriptTag({url: `${origin}/js/${file}`});
        }
        await page.evaluate(words => {
            const esc = value => String(value == null ? '' : value).replace(/[&<>"']/g, ch => ({'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'}[ch]));
            document.body.className = 'desktop-body';
            document.body.dataset.theme = 'standard';
            document.body.dataset.scheme = 'dark';
            window.ChessApp.render(document.querySelector('#host'), 'chess', {
                t: (key, vars = {}) => { let text = words[key] || key; for (const [k, v] of Object.entries(vars)) text = text.replaceAll('{' + k + '}', v); return text; },
                esc,
                iconMarkup: (key, fallback, cls) => `<span class="${esc(cls || '')}">${esc(fallback || key || '')}</span>`,
                api: async () => ({}),
                notify: () => {},
                setWindowMenus: () => {},
                clearWindowMenus: () => {}
            }, {});
        }, words);
        await page.locator('.cm-chessboard').first().waitFor({state: 'attached', timeout: 15000});
        await page.waitForTimeout(600);

        const side = page.locator('.vd-chess-side');
        const sideState = await side.evaluate(node => ({
            overflow: node.scrollHeight - node.clientHeight,
            display: getComputedStyle(node).display,
            children: [...node.children].map(child => `${child.className.replace('vd-chess-', '')}:${child.getBoundingClientRect().height}:${getComputedStyle(child).display}`)
        }));
        assert.equal(sideState.display, 'flex', 'the chess sidebar must be a flex column');
        if (width >= 1000) {
            assert(sideState.overflow <= 1, `the chess sidebar must not clip its content, overflow ${sideState.overflow}px (${sideState.children.join(', ')})`);
        } else {
            /* The stacked layout gives the sidebar its own scroll instead of
               cutting the content off. */
            assert.equal(await side.evaluate(node => getComputedStyle(node).overflowY), 'auto', 'the stacked sidebar must scroll instead of clipping');
        }
        const visiblePanels = await page.locator('[data-panel]').evaluateAll(nodes => nodes.filter(node => getComputedStyle(node).display !== 'none').length);
        assert.equal(visiblePanels, 1, 'only the active tab panel may be visible');
        assert.equal(await page.locator('.vd-chess-replay').evaluate(node => getComputedStyle(node).display), 'none', 'the replay bar must stay hidden until a game is reviewed');
        assert.equal(await page.locator('.vd-chess-settings').evaluate(node => node.open), false, 'the game settings start folded away');

        const square = await page.locator('.vd-chess-board-frame').evaluate(node => { const rect = node.getBoundingClientRect(); return [rect.width, rect.height]; });
        assert(Math.abs(square[0] - square[1]) < 2, `the 2D board must stay square, got ${square.join('x')}`);
        assert(square[0] > 180, `the 2D board must keep a usable size, got ${square[0]}`);

        await page.locator('[data-chess-view="3d"]').click();
        await page.waitForTimeout(1800);
        const canvas = await page.locator('.vd-chess-3d-canvas').evaluate(node => { const rect = node.getBoundingClientRect(); return [rect.width, rect.height]; });
        const shell = await page.locator('.vd-chess-board-shell').evaluate(node => { const rect = node.getBoundingClientRect(); return [rect.width, rect.height]; });
        assert(canvas[0] > square[0] * 1.4, `the 3D board must use the free width, got ${canvas[0]} for a ${square[0]} square board`);
        assert(Math.abs(canvas[0] - shell[0]) < 60, `the 3D canvas must fill the board area, got ${canvas[0]} of ${shell[0]}`);
        await page.screenshot({path: path.join(report, `chess-3d-${width}.png`)});

        await page.locator('[data-chess-view="2d"]').click();
        await page.waitForTimeout(800);
        const back = await page.locator('.vd-chess-board-frame').evaluate(node => { const rect = node.getBoundingClientRect(); return [rect.width, rect.height]; });
        assert(Math.abs(back[0] - square[0]) < 2 && Math.abs(back[1] - square[1]) < 2, `the board must return to its square size, got ${back.join('x')}`);
        assert.equal(errors.length, 0, `chess page errors: ${errors.join(' | ')}`);
        await page.close();
    }
    console.log('Chess browser: sidebar fits, panels stay exclusive, 2D square and 3D wide board passed.');
} finally {
    await browser.close();
    server.close();
}
