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
        await page.route('**/api/desktop/chess/commentary', async route => {
            const req = JSON.parse(route.request().postData() || '{}');
            await route.fulfill({
                status: 200,
                contentType: 'application/json',
                body: JSON.stringify({
                    category: req.category || 'opening_e4',
                    variant: 0,
                    language: 'de',
                    text: `Sarkastischer Kommentar (${req.category || 'move'}) auf ${req.square || 'e4'}.`,
                    audio_unavailable: true
                })
            });
        });
        await page.route('**/api/desktop/chess/agent-move', route => route.fulfill({status: 200, contentType: 'application/json', body: JSON.stringify({ok: true})}));
        await page.goto(origin);
        await page.addStyleTag({url: `${origin}/css/desktop-shell.bundle.css`});
        await page.addStyleTag({url: `${origin}/css/cm-chessboard.css`});
        await page.addStyleTag({url: `${origin}/css/desktop-app-chess.css`});
        for (const file of ['vendor/three.min.js', 'vendor/GLTFLoader.min.js', 'vendor/OrbitControls.min.js', 'desktop/apps/chess-engine.js', 'desktop/apps/chess-agent.js', 'desktop/apps/chess-commentary.js', 'desktop/apps/chess-fx.js', 'desktop/apps/chess-table.js', 'desktop/apps/chess-3d-stage.js', 'desktop/apps/chess-3d.js', 'desktop/apps/chess.js']) {
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

        // The table around the board: player plates, engraved rim coordinates and the lit surface.
        const plate = name => page.locator(`[data-plate="${name}"]`);
        const plateName = name => plate(name).locator('[data-plate-name]').textContent();
        assert.equal(await plate('top').getAttribute('data-clock'), 'black', 'black sits on top of a board seen by white');
        assert.equal(await plate('bottom').getAttribute('data-clock'), 'white', 'white sits at the bottom of a board seen by white');
        assert.equal(await plateName('bottom'), words['desktop.chess_you'], 'the bottom plate names the human player');
        assert.equal(await plateName('top'), `${words['desktop.chess_opponent_computer']} · ${words['desktop.chess_strength']} 8`, 'the top plate names the engine and its level');
        assert.equal(await page.locator('.cm-chessboard .coordinates text').count(), 0, 'the squares carry no coordinates; they are engraved in the rim');
        const rim = () => page.locator('[data-rim-coords] span').evaluateAll(nodes => nodes.map(node => node.textContent));
        let rimText = await rim();
        assert.equal(rimText.length, 32, 'the rim carries every file and rank on both sides');
        /* The rim lists, per square, the file above, the file below, the rank
           left and the rank right; every second group of four is one column. */
        assert.deepEqual(rimText.filter((_, i) => i % 4 === 0), ['A', 'B', 'C', 'D', 'E', 'F', 'G', 'H'], 'files read left to right for white');
        assert.deepEqual(rimText.filter((_, i) => i % 4 === 1), ['A', 'B', 'C', 'D', 'E', 'F', 'G', 'H'], 'the far rim repeats the files upright');
        assert.deepEqual(rimText.filter((_, i) => i % 4 === 2), ['8', '7', '6', '5', '4', '3', '2', '1'], 'ranks count down from the top for white');
        assert.deepEqual(rimText.filter((_, i) => i % 4 === 3), ['8', '7', '6', '5', '4', '3', '2', '1'], 'the right rim repeats the ranks');
        assert.equal(await page.locator('[data-plate-clock]').first().isVisible(), false, 'the clocks stay hidden while the clock is off');

        const frameBox = () => page.locator('.vd-chess-board-frame').evaluate(node => { const r = node.getBoundingClientRect(); return {top: r.top, bottom: r.bottom, left: r.left, right: r.right, width: r.width, height: r.height}; });
        const plateBox = name => plate(name).evaluate(node => { const r = node.getBoundingClientRect(); return {top: r.top, bottom: r.bottom, left: r.left, right: r.right}; });
        let frame = await frameBox();
        const topBox = await plateBox('top');
        const bottomBox = await plateBox('bottom');
        /* The board is square and the pane is wide, so the board must take the
           free height of the shell. A toolbar that wraps or a chrome row that
           grows would shrink it, so the board has to fill the board area. */
        const shellBox = await page.locator('.vd-chess-board-shell').evaluate(node => { const r = node.getBoundingClientRect(); return [r.width, r.height]; });
        assert(frame.height >= shellBox[1] * 0.85, `the 2D board must fill the board area, got ${frame.height} of ${shellBox[1]}`);
        const headBox = await page.locator('.vd-chess-pane-head').evaluate(node => { const r = node.getBoundingClientRect(); return [r.width, r.height]; });
        assert(headBox[1] < 48, `the toolbar must stay on one row, got ${headBox[1]}px`);
        assert(headBox[0] <= shellBox[0], 'the toolbar must stay inside the board area');
        if (width > 820) {
            assert(Math.abs(topBox.left - frame.left) < 2 && Math.abs(topBox.right - frame.right) < 2, `the top plate follows the board width, got ${topBox.left}-${topBox.right} for ${frame.left}-${frame.right}`);
            assert(Math.abs(bottomBox.left - frame.left) < 2 && Math.abs(bottomBox.right - frame.right) < 2, 'the bottom plate follows the board width');
            assert(topBox.bottom <= frame.top && bottomBox.top >= frame.bottom, 'the plates frame the board without covering it');
        } else {
            /* The stacked layout puts both plates in one row under the board. */
            assert(Math.abs(topBox.top - bottomBox.top) < 2 && topBox.left < bottomBox.left, 'the stacked plates share one row');
            assert(bottomBox.top >= frame.bottom - 1, `the stacked board must stay whole above the plates, got bottom ${frame.bottom} and plate top ${bottomBox.top}`);
        }

        await page.locator('[data-flip]').click();
        await page.waitForTimeout(600);
        assert.equal(await plate('top').getAttribute('data-clock'), 'white', 'flipping moves white to the top plate');
        rimText = await rim();
        assert.deepEqual(rimText.filter((_, i) => i % 4 === 0), ['H', 'G', 'F', 'E', 'D', 'C', 'B', 'A'], 'flipping mirrors the files');
        assert.deepEqual(rimText.filter((_, i) => i % 4 === 2), ['1', '2', '3', '4', '5', '6', '7', '8'], 'flipping mirrors the ranks');
        await page.locator('[data-flip]').click();
        await page.waitForTimeout(600);

        await page.locator('[data-settings]').evaluate(node => { node.open = true; });
        await page.locator('[data-board-skin]').selectOption('marble');
        assert.equal(await page.locator('.vd-chess').getAttribute('data-chess-skin'), 'marble', 'the skin reaches the app root');
        assert.equal(await page.locator('svg.cm-chessboard').evaluate(node => node.classList.contains('marble')), true, 'the skin stays on the vendor board class');
        await page.locator('[data-board-skin]').selectOption('green');

        await page.locator('[data-clock-select]').selectOption('180');
        await page.waitForTimeout(400);
        assert.equal(await page.locator('[data-plate-clock]').first().isVisible(), true, 'a timed game shows the clocks in the plates');
        for (const text of await page.locator('[data-plate-clock]').allTextContents()) assert.match(text, /^\d\d:\d\d$/, `a plate clock reads mm:ss, got "${text}"`);
        await page.locator('[data-clock-select]').selectOption('0');
        await page.waitForTimeout(400);
        assert.equal(await page.locator('[data-plate-clock]').first().isVisible(), false, 'an untimed game hides the clocks again');

        await page.locator('[data-mode]').selectOption('local');
        assert.equal(await plateName('top'), words['desktop.chess_color_black'], 'local play names the plates by colour');
        assert.equal(await plateName('bottom'), words['desktop.chess_color_white'], 'local play names the plates by colour');

        // Selecting a piece rings it and marks its legal moves.
        await page.locator('.vd-chess-board [data-square="e2"]').first().click();
        await page.waitForTimeout(250);
        assert.equal(await page.locator('.marker-circle-primary').count(), 1, 'the selected piece carries a ring');
        assert.equal(await page.locator('.marker-dot').count(), 2, 'the pawn shows its two legal moves');
        await page.locator('.vd-chess-board [data-square="e4"]').first().click();
        await page.waitForFunction(() => document.querySelector('[data-moves-count]').textContent === '1');

        const move = async (from, to) => {
            await page.locator(`.vd-chess-board [data-square="${from}"]`).first().click();
            await page.locator(`.vd-chess-board [data-square="${to}"]`).first().click();
            await page.waitForTimeout(350); // cm-chessboard animates position changes for 300 ms.
        };
        assert.equal(await page.locator('[data-undo]').isDisabled(), false, 'undo must enable after a move');
        await page.locator('[data-undo]').click();
        await page.waitForFunction(() => document.querySelector('[data-moves-count]').textContent === '0');
        await page.waitForTimeout(350);
        assert.equal(await page.locator('[data-undo]').isDisabled(), true, 'undo must disable after returning to the initial position');

        await move('e2', 'e4');
        await move('e7', 'e5');
        await page.waitForFunction(() => document.querySelector('[data-moves-count]').textContent === '2');
        assert.equal(await page.locator('.marker-square').count(), 2, 'the last move tints both of its squares');
        assert.equal(await page.locator('[data-jump][aria-current="true"]').getAttribute('data-jump'), '1', 'the live board must mark its latest move');
        const firstMove = page.locator('[data-jump="0"]');
        await firstMove.focus();
        await page.keyboard.press('Enter');
        assert.equal(await page.locator('[data-jump="0"]').getAttribute('aria-current'), 'true', 'reviewing a move must mark it current');
        assert.equal(await page.evaluate(() => document.activeElement.dataset.jump), '0', 'reviewing must preserve keyboard focus in the move list');
        await page.keyboard.press('Tab');
        assert.equal(await page.evaluate(() => document.activeElement.dataset.jump), '1', 'Tab must advance to the next move button');
        await page.keyboard.press('Enter');
        assert.equal(await page.locator('[data-jump="1"]').getAttribute('aria-current'), 'true', 'the next move button must select its move');

        const moveTab = page.locator('[data-tab="moves"]');
        await moveTab.focus();
        await page.keyboard.press('ArrowRight');
        assert.equal(await page.locator('[data-tab="captured"]').getAttribute('aria-selected'), 'true', 'ArrowRight must select the next chess tab');
        assert.equal(await page.locator('[data-panel="captured"]').isVisible(), true, 'the selected chess tab must reveal its panel');
        await page.keyboard.press('ArrowRight');
        await page.keyboard.press('ArrowRight');
        assert.equal(await page.locator('[data-tab="moves"]').getAttribute('aria-selected'), 'true', 'the chess tabs must wrap at the end');
        await page.locator('[data-review-last]').click();
        assert.equal(await page.locator('[data-review-label]').textContent(), words['desktop.chess_live'], 'the last replay control must restore the live position');

        const square = await page.locator('.vd-chess-board-frame').evaluate(node => { const rect = node.getBoundingClientRect(); return [rect.width, rect.height]; });
        assert(Math.abs(square[0] - square[1]) < 2, `the 2D board must stay square, got ${square.join('x')}`);
        assert(square[0] > 180, `the 2D board must keep a usable size, got ${square[0]}`);

        /* The remark under the board must never change the board size: while it
           lived in the status ribbon a two-line remark grew that ribbon and
           shrank the board mid-game. */
        await page.locator('[data-comment]').evaluate(node => {
            node.textContent = 'Ein langer Kommentar des Agenten, der im alten Ribbon zwei Zeilen belegt hat und das Brett dadurch kleiner gemacht hat.';
        });
        await page.waitForTimeout(300);
        const afterRemark = await page.locator('.vd-chess-board-frame').evaluate(node => node.getBoundingClientRect().width);
        assert(Math.abs(afterRemark - square[0]) < 1, `the board must not shrink when the remark grows, got ${square[0]} -> ${afterRemark}`);

        // Verify sound & optional Phonon commentary toggles and contextual move remark
        await page.locator('[data-sound-toggle]').click();
        assert.equal(await page.locator('[data-sound-toggle]').getAttribute('aria-pressed'), 'false', 'sound toggle must mute on click');
        await page.locator('[data-sound-toggle]').click();
        assert.equal(await page.locator('[data-sound-toggle]').getAttribute('aria-pressed'), 'true', 'sound toggle must unmute on second click');

        assert.equal(await page.locator('[data-commentary-toggle]').getAttribute('aria-pressed'), 'false', 'voice commentary starts opt-in off');
        await page.locator('[data-commentary-toggle]').click();
        assert.equal(await page.locator('[data-commentary-toggle]').getAttribute('aria-pressed'), 'true', 'voice commentary enables on click');
        await move('g1', 'h3');
        await page.waitForFunction(() => (document.querySelector('[data-comment]')?.textContent || '').includes('knight_rim'));

        // Captures fill the trays: ...Nc6 d4 Nxd4 Qxd4 trades a pawn for a knight.
        await move('b8', 'c6');
        await move('d2', 'd4');
        await move('c6', 'd4');
        await move('d1', 'd4');
        await page.waitForFunction(() => document.querySelector('[data-moves-count]').textContent === '7');
        const tray = name => plate(name).locator('[data-plate-tray] .vd-chess-tray-piece').allTextContents();
        assert.deepEqual(await tray('bottom'), ['♞'], 'white has taken a knight');
        assert.deepEqual(await tray('top'), ['♟'], 'black has taken a pawn');
        assert.equal(await plate('bottom').locator('[data-plate-lead]').isVisible(), true, 'the leading side shows its material lead');
        assert.equal(await plate('bottom').locator('[data-plate-lead]').textContent(), '+2', 'a knight against a pawn is two points up');
        assert.equal(await plate('top').locator('[data-plate-lead]').isVisible(), false, 'the trailing side shows no lead');
        await page.screenshot({path: path.join(report, `chess-2d-${width}.png`)});
        await page.locator('[data-commentary-toggle]').click();

        await page.locator('[data-chess-view="3d"]').click();
        await page.waitForTimeout(1800);
        const canvas = await page.locator('.vd-chess-3d-canvas').evaluate(node => { const rect = node.getBoundingClientRect(); return [rect.width, rect.height]; });
        const shell = await page.locator('.vd-chess-board-shell').evaluate(node => { const rect = node.getBoundingClientRect(); return [rect.width, rect.height]; });
        assert(canvas[0] > square[0] * 1.4, `the 3D board must use the free width, got ${canvas[0]} for a ${square[0]} square board`);
        assert(Math.abs(canvas[0] - shell[0]) < 60, `the 3D canvas must fill the board area, got ${canvas[0]} of ${shell[0]}`);
        assert.equal(await page.locator('.vd-chess-surface').evaluate(node => getComputedStyle(node).display), 'none', 'the lit 2D surface steps aside for the 3D scene');
        await page.screenshot({path: path.join(report, `chess-3d-${width}.png`)});

        await page.locator('[data-chess-view="2d"]').click();
        await page.waitForTimeout(800);
        const back = await page.locator('.vd-chess-board-frame').evaluate(node => { const rect = node.getBoundingClientRect(); return [rect.width, rect.height]; });
        assert(Math.abs(back[0] - square[0]) < 2 && Math.abs(back[1] - square[1]) < 2, `the board must return to its square size, got ${back.join('x')}`);
        assert.equal(errors.length, 0, `chess page errors: ${errors.join(' | ')}`);
        await page.close();
    }
    console.log('Chess browser: sidebar fits, panels stay exclusive, plates and rim follow the board, captures fill the trays, 2D square and 3D wide board passed.');
} finally {
    await browser.close();
    server.close();
}
