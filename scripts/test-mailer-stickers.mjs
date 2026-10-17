import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {readFile, mkdir} from 'node:fs/promises';
import {createRequire} from 'node:module';
import path from 'node:path';
import {fileURLToPath} from 'node:url';

const require = createRequire(import.meta.url);
const {chromium} = require(process.env.AURAGO_PLAYWRIGHT || 'playwright');
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const words = {...JSON.parse(await readFile(path.join(root, 'ui/lang/desktop/en.json'), 'utf8')), ...JSON.parse(await readFile(path.join(root, 'ui/lang/mailer/en.json'), 'utf8'))};
const report = path.join(root, 'reports/mailer/stickers');
await mkdir(report, {recursive:true});
const server = createServer(async (req, res) => {
    const url = new URL(req.url, 'http://localhost');
    if (url.pathname === '/') { res.setHeader('Content-Type', 'text/html'); res.end('<!doctype html><meta charset="utf-8"><style>html,body{margin:0;height:100%}#host{height:100%}</style><div id="host"></div>'); return; }
    try {
        const file = path.resolve(root, 'ui', '.' + url.pathname);
        if (!file.startsWith(path.join(root, 'ui') + path.sep)) throw Error('invalid path');
        res.setHeader('Content-Type', ({'.js':'text/javascript','.css':'text/css','.png':'image/png','.woff2':'font/woff2'})[path.extname(file)] || 'application/octet-stream');
        res.end(await readFile(file));
    } catch { res.statusCode = 404; res.end(); }
});
await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
const origin = `http://127.0.0.1:${server.address().port}`;
const browser = await chromium.launch({headless:true, channel:process.env.PLAYWRIGHT_CHANNEL || 'chrome'});
try {
    for (const theme of ['standard','fruity']) for (const mode of ['dark','light']) for (const width of [1280,390]) {
        const page = await browser.newPage({viewport:{width,height:1000}});
        const errors = [], requests = [], saves = [];
        let draft, serial = 0, failUpload = false, sends = 0;
        page.on('pageerror', error => errors.push(error.message));
        page.on('request', request => requests.push(request.url()));
        await page.route('**/api/desktop/mailer/**', async route => {
            const request = route.request(), url = new URL(request.url());
            const endpoint = url.pathname.replace('/api/desktop/mailer/', '');
            const json = body => route.fulfill({status:200,contentType:'application/json',body:JSON.stringify(body)});
            if (/attachments$/.test(endpoint)) {
                if (failUpload) { failUpload = false; return route.fulfill({status:500,contentType:'application/json',body:'{"error":"upload_failed"}'}); }
                const data = request.postDataBuffer();
                const filename = /filename="([^"]+)"/.exec(data.toString('latin1'))?.[1];
                draft = {...draft, revision:draft.revision+1, attachments:[...draft.attachments, {id:'blob_' + ++serial, filename, content_type:'image/png', size:data.length}]};
                return json(draft);
            }
            if (/attachments\/[^/]+$/.test(endpoint) && request.method() === 'DELETE') {
                draft = {...draft, revision:draft.revision+1, attachments:draft.attachments.filter(item => item.id !== endpoint.split('/').at(-1))};
                return json(draft);
            }
            if (endpoint.startsWith('drafts') && ['POST','PUT'].includes(request.method())) {
                const payload = request.postDataJSON();
                draft = {...payload,id:'draft-fixture',revision:(draft?.revision || 0)+1};
                saves.push(structuredClone(draft));
                return json(draft);
            }
            if (endpoint === 'outbox') { sends++; return json({id:'queued',status:'scheduled'}); }
            return json({items:[]});
        });
        await page.goto(origin);
        for (const file of ['desktop-shell.bundle','desktop-app-mailer','desktop-stickers']) await page.addStyleTag({url:origin+'/css/'+file+'.css'});
        for (const file of ['vendor/marked.min','vendor/purify.min','desktop/shared/sticker-catalog','desktop/shared/sticker-library','desktop/apps/mailer-api','desktop/apps/mailer-stickers','desktop/apps/mailer-compose']) await page.addScriptTag({url:origin+'/js/'+file+'.js'});
        await page.evaluate(({words,theme,mode}) => {
            document.body.className = 'desktop-body'; document.body.dataset.theme = theme; document.documentElement.dataset.theme = mode;
            document.documentElement.style.setProperty('--surface-color', mode === 'light' ? '#f5f6fa' : '#111823');
            document.documentElement.style.setProperty('--text-color', mode === 'light' ? '#202b3a' : '#edf2f8');
            document.documentElement.style.setProperty('--text-muted', mode === 'light' ? '#526174' : '#9aa8ba');
            window.BUILD_VERSION = 'fixture';
            window.AuraLazyAssets = {versionedURL: url => url+'?v=fixture'};
            window.ctx = {
                state:{accounts:[{id:'email:fixture',address:'sender@example.test'}]},
                t:key => words[key] || key,
                api:async (url, options) => { const response = await fetch(url,options); const body = await response.json(); if (!response.ok) throw Object.assign(new Error(body.error),{status:response.status}); return body; },
                confirmDialog:async () => true, setWindowBeforeClose:(_,callback) => window.beforeClose = callback,
                registerWindowCleanup:() => {}
            };
            window.openComposer = draft => window.composer = MailerCompose.render(document.querySelector('#host'),'mail',ctx,{draft,accountId:'email:fixture'});
            openComposer();
        }, {words,theme,mode});
        const editor = page.locator('[data-mailer-richtext]');
        await editor.waitFor();
        await page.evaluate(() => {
            composer.setMarkdown('Hello world');
            const view = composer.editorView;
            view.dispatch(view.state.tr.setSelection(composer.editorLib.TextSelection.create(view.state.doc, 6)));
        });
        await page.locator('[data-stickers-open]').focus(); await page.keyboard.press('Enter');
        assert.equal(await page.locator('[data-sticker-id]').count(),100);
        assert(!requests.some(url => /\/img\/stickers\/(?!thumbs)/.test(url)), 'picker fetched full-size images');
        await page.getByRole('searchbox').fill('no-such-sticker');
        assert.equal(await page.locator('[data-sticker-id]').count(),0);
        assert.equal(await page.locator('.sticker-picker-status').textContent(),words['stickers.empty']);
        await page.getByRole('searchbox').fill('');
        await page.locator('.sticker-picker-controls select').first().selectOption('animals');
        await page.locator('.sticker-picker-controls select').last().selectOption('clay');
        assert.equal(await page.locator('[data-sticker-id]').count(),1);
        await page.screenshot({path:path.join(report,`picker-${theme}-${mode}-${width}.png`)});
        const editWhileLoading = theme === 'standard' && mode === 'dark' && width === 1280;
        let releaseSticker, stickerStarted;
        const stickerGate = new Promise(resolve => releaseSticker = resolve), stickerReady = new Promise(resolve => stickerStarted = resolve);
        if (editWhileLoading) await page.route('**/img/stickers/animals-cat.png?*', async route => { stickerStarted(); await stickerGate; await route.continue(); });
        await page.locator('[data-sticker-id="animals-cat"]').focus(); await page.keyboard.press('Enter');
        if (editWhileLoading) {
            await stickerReady;
            await page.evaluate(() => { const view = composer.editorView; view.dispatch(view.state.tr.insertText('Extra ',1)); });
            releaseSticker();
        }
        await page.waitForFunction(() => document.querySelector('.vd-mailer-sticker-image')?.naturalWidth === 192);
        await page.evaluate(() => composer.uploadTask);
        if (editWhileLoading) assert(/Extra Hello.*!\[Cat\]/.test(draft.text),'insertion position did not follow edits during loading');
        assert(draft.text.includes('![Cat](cid:blob_1@aurago)'),draft.text);
        assert(draft.text.indexOf('![Cat]') < draft.text.indexOf('world'),'sticker ignored insertion point');
        assert(draft.html.includes('src="cid:blob_1@aurago"') && draft.html.includes('width="192"'),draft.html);
        assert(!draft.html.includes('/img/stickers/'),'outgoing HTML depends on local resources');
        assert.equal(draft.attachments[0].inline,true);
        assert.equal(draft.attachments[0].content_id,'blob_1@aurago');
        assert.equal(draft.attachments[0].filename,'sticker-animals-cat.png');
        assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),true,'horizontal overflow');
        await editor.focus(); await page.keyboard.press('Control+z');
        assert.equal(await editor.locator('img').count(),0,'sticker cannot be undone');
        await page.keyboard.press('Control+Shift+z');
        assert.equal(await editor.locator('img').count(),1,'sticker cannot be redone');
        await page.locator('[data-save-now]').click(); await page.evaluate(() => composer.saveQueue);
        await page.evaluate(draft => openComposer(draft),draft);
        await editor.waitFor();
        await page.waitForFunction(() => document.querySelector('.vd-mailer-sticker-image')?.naturalWidth === 192);
        assert.equal(await page.evaluate(() => composer.preserveHTML),false,'own sticker draft became foreign HTML');
        await page.screenshot({path:path.join(report,`compose-${theme}-${mode}-${width}.png`)});
        await page.locator('[data-stickers-open]').click();
        await page.getByRole('searchbox').press('Escape');
        assert.equal(await page.locator('[data-mailer-stickers]').isVisible(),false);
        if (width === 1280 && mode === 'dark') {
            await page.locator('[data-stickers-open]').click();
            await page.route('**/img/stickers/food-pizza.png?*', route => route.fulfill({status:404,body:''}));
            await page.locator('[data-sticker-id="food-pizza"]').click();
            await page.getByText(words['stickers.failed'],{exact:true}).waitFor();
            assert.equal(draft.attachments.length,1,'failed fetch uploaded a file');
            await page.unroute('**/img/stickers/food-pizza.png?*');
            failUpload = true;
            await page.locator('[data-sticker-id="food-pizza"]').click();
            await page.locator('[data-retry-upload]').waitFor();
            assert.equal(await page.locator('[data-send]').isDisabled(),true);
            await page.locator('[data-retry-upload]').click();
            await page.waitForFunction(() => document.querySelectorAll('.vd-mailer-sticker-image').length === 2);
            await page.evaluate(() => composer.uploadTask);
            assert.equal(draft.attachments.length,2,'retry duplicated attachment');
            assert(draft.attachments.every(item => item.inline),'retry lost inline metadata');
            await page.locator('[data-remove-attachment="blob_1"]').click();
            await page.evaluate(() => composer.attachmentTask);
            assert.equal(await editor.locator('.vd-mailer-sticker-image').count(),1,'removing the attachment left its image behind');
            assert(!draft.html.includes('blob_1@aurago'),'removed sticker remains in outgoing HTML');
            await page.locator('[data-recipient-input="to"]').fill('recipient@example.test'); await page.keyboard.press('Enter');
            {
                let release, started;
                const gate = new Promise(resolve => release = resolve), ready = new Promise(resolve => started = resolve);
                await page.route('**/img/stickers/nature-sun.png?*', async route => { started(); await gate; await route.fulfill({status:404,body:''}); });
                await page.locator('[data-stickers-open]').click();
                await page.locator('[data-sticker-id="nature-sun"]').click(); await ready;
                await page.locator('[data-send]').click();
                assert.equal(sends,0,'send overtook sticker loading');
                release();
                await page.waitForFunction(() => !composer.sending);
                assert.equal(sends,0,'failed sticker load silently queued email');
                await page.unroute('**/img/stickers/nature-sun.png?*');
                await page.getByRole('searchbox').press('Escape');
            }
            // A closed editor must not upload the image when a delayed fetch completes.
            let release, started;
            const gate = new Promise(resolve => release = resolve), ready = new Promise(resolve => started = resolve);
            await page.route('**/img/stickers/nature-sun.png?*', async route => { started(); await gate; await route.continue().catch(() => {}); });
            await page.locator('[data-stickers-open]').click();
            const before = saves.length;
            await page.locator('[data-sticker-id="nature-sun"]').click();
            await ready;
            await page.evaluate(() => MailerCompose.dispose('mail'));
            release();
            await page.evaluate(() => composer.uploadTask);
            assert.equal(saves.length,before,'closed composer saved delayed sticker');
            await page.evaluate(draft => openComposer(draft),draft); await editor.waitFor();
        }
        await page.locator('[data-recipient-input="to"]').fill('recipient@example.test'); await page.keyboard.press('Enter');
        await page.locator('[data-send]').click();
        await page.waitForFunction(() => composer.committed);
        assert.equal(sends,1,'send was not queued exactly once');
        assert.equal(await page.locator('[data-stickers-open]').isDisabled(),true,'queued draft stays editable');
        if (theme === 'standard' && mode === 'light' && width === 390) {
            await page.route('**/js/vendor/notes/engine.js?v=fallback', route => route.fulfill({status:404,body:''}));
            await page.evaluate(() => { window.BUILD_VERSION = 'fallback'; openComposer(); });
            await page.locator('[data-body-fallback]').waitFor({state:'visible'});
            await page.locator('[data-body-fallback]').fill('Fallback ');
            await page.locator('[data-stickers-open]').click();
            await page.locator('[data-sticker-id="animals-cat"]').click();
            await page.waitForFunction(() => document.querySelector('[data-body-fallback]').value.includes('![Cat](cid:'));
            await page.evaluate(() => composer.uploadTask);
            assert(draft.html.includes('width="192"'),'source editor lost sticker HTML');
            assert.equal(await page.evaluate(() => beforeClose()),true);
            await page.evaluate(draft => { ctx.readonly = true; openComposer(draft); },draft);
            assert.equal(await page.locator('[data-stickers-open]').isDisabled(),true,'read-only composer allows stickers');
            await page.evaluate(() => { ctx.readonly = false; openComposer({html:'<table><tr><td>Original HTML</td></tr></table>',text:'![Tracking](https://tracker.invalid/pixel)'}); });
            await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
            assert(!requests.some(url => url.includes('tracker.invalid')),'HTML preparation loaded an external tracking image');
            assert.equal(await page.locator('[data-stickers-open]').isDisabled(),true,'foreign HTML was silently converted');
        }
        assert.deepEqual(errors,[],`${theme}/${mode}/${width}`);
        await page.evaluate(() => MailerCompose.dispose('mail')); await page.close();
    }
    console.log('Mailer stickers: 8 theme/width cases, selection, CID drafts, undo/redo, failure/retry, lifecycle and queue passed.');
} finally { await browser.close(); await new Promise(resolve => server.close(resolve)); }
